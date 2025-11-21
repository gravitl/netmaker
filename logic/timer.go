package logic

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gravitl/netmaker/logger"
	"golang.org/x/exp/slog"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
)

// == Constants ==

// How long to wait before sending telemetry to server (24 hours)
const timer_hours_between_runs = 24

// HookManagerCh - channel to add any new hooks
var HookManagerCh = make(chan models.HookDetails, 3)

// HookCommandCh - channel to send commands to hooks (reset/stop)
var HookCommandCh = make(chan models.HookCommand, 10)

// hookInfo - tracks running hooks
type hookInfo struct {
	cancelFunc context.CancelFunc
	resetCh    chan struct{}
	interval   time.Duration
}

// runningHooks - map of hook ID to hook info
var runningHooks = make(map[string]*hookInfo)
var hooksMutex sync.RWMutex

// == Public ==

// TimerCheckpoint - Checks if 24 hours has passed since telemetry was last sent. If so, sends telemetry data to posthog
func TimerCheckpoint() error {
	// get the telemetry record in the DB, which contains a timestamp
	telRecord, err := FetchTelemetryRecord()
	if err != nil {
		return err
	}
	sendtime := time.Unix(telRecord.LastSend, 0).Add(time.Hour * time.Duration(timer_hours_between_runs))
	// can set to 2 minutes for testing
	// sendtime := time.Unix(telRecord.LastSend, 0).Add(time.Minute * 2)
	enoughTimeElapsed := time.Now().After(sendtime)
	// if more than 24 hours has elapsed, send telemetry to posthog
	if enoughTimeElapsed {
		// run any time hooks
		runHooks()
		return setTelemetryTimestamp(&telRecord)

	}
	return nil
}

// AddHook - adds a hook function to run every 24hrs
func AddHook(ifaceToAdd interface{}) {
	timeHooks = append(timeHooks, ifaceToAdd)
}

// ResetHook - resets the timer for a hook with the given ID
func ResetHook(hookID string) {
	HookCommandCh <- models.HookCommand{
		ID:      hookID,
		Command: models.HookCommandReset,
	}
}

// StopHook - stops a hook with the given ID
func StopHook(hookID string) {
	HookCommandCh <- models.HookCommand{
		ID:      hookID,
		Command: models.HookCommandStop,
	}
}

// GetRunningHooks - returns a list of currently running hook IDs
func GetRunningHooks() []string {
	hooksMutex.RLock()
	defer hooksMutex.RUnlock()

	ids := make([]string, 0, len(runningHooks))
	for id := range runningHooks {
		ids = append(ids, id)
	}
	return ids
}

// StartHookManager - listens on `HookManagerCh` to run any hook and `HookCommandCh` for commands
func StartHookManager(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			slog.Info("## Stopping Hook Manager")
			// Cancel all running hooks
			hooksMutex.Lock()
			for _, info := range runningHooks {
				info.cancelFunc()
			}
			runningHooks = make(map[string]*hookInfo)
			hooksMutex.Unlock()
			return
		case newhook := <-HookManagerCh:
			hookID := newhook.ID
			if hookID == "" {
				hookID = uuid.New().String()
			}

			// Check if hook with this ID already exists
			hooksMutex.Lock()
			if existingHook, exists := runningHooks[hookID]; exists {
				// Stop existing hook before replacing
				existingHook.cancelFunc()
				delete(runningHooks, hookID)
			}

			// Create context for this hook
			hookCtx, cancelFunc := context.WithCancel(ctx)
			resetCh := make(chan struct{}, 1)

			info := &hookInfo{
				cancelFunc: cancelFunc,
				resetCh:    resetCh,
				interval:   newhook.Interval,
			}
			runningHooks[hookID] = info
			hooksMutex.Unlock()

			wg.Add(1)
			go addHookWithInterval(hookCtx, wg, hookID, newhook.Hook, newhook.Params, newhook.Interval, resetCh)
		case cmd := <-HookCommandCh:
			hooksMutex.Lock()
			info, exists := runningHooks[cmd.ID]
			hooksMutex.Unlock()

			if !exists {
				slog.Warn("hook not found", "hook_id", cmd.ID)
				continue
			}

			switch cmd.Command {
			case models.HookCommandReset:
				// Send reset signal
				select {
				case info.resetCh <- struct{}{}:
					slog.Info("reset signal sent to hook", "hook_id", cmd.ID)
				default:
					// Channel is full, skip
				}
			case models.HookCommandStop:
				// Stop the hook
				info.cancelFunc()
				hooksMutex.Lock()
				delete(runningHooks, cmd.ID)
				hooksMutex.Unlock()
				slog.Info("hook stopped", "hook_id", cmd.ID)
			}
		}
	}
}

func addHookWithInterval(ctx context.Context, wg *sync.WaitGroup, hookID string, hook models.HookFunc, params []interface{}, interval time.Duration, resetCh chan struct{}) {
	defer wg.Done()
	defer func() {
		hooksMutex.Lock()
		delete(runningHooks, hookID)
		hooksMutex.Unlock()
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-resetCh:
			// Reset the timer by stopping the old ticker and creating a new one
			ticker.Stop()
			ticker = time.NewTicker(interval)
			slog.Info("hook timer reset", "hook_id", hookID)
		case <-ticker.C:
			if err := hook(params...); err != nil {
				slog.Error("error running hook", "hook_id", hookID, "error", err.Error())
			}
		}
	}
}

// WrapHook - wraps a parameterless hook function to be compatible with HookFunc
// This allows backward compatibility with existing hooks that don't accept parameters
func WrapHook(hook func() error) models.HookFunc {
	return func(...interface{}) error {
		return hook()
	}
}

// == private ==

// timeHooks - functions to run once a day, functions must take no parameters
var timeHooks = []interface{}{
	loggerDump,
	sendTelemetry,
}

func loggerDump() error {
	// TODO use slog?
	logger.DumpFile(fmt.Sprintf("data/netmaker.log.%s", time.Now().Format(logger.TimeFormatDay)))
	return nil
}

// runHooks - runs the functions currently in the timeHooks data structure
func runHooks() {
	for _, hook := range timeHooks {
		if err := hook.(func() error)(); err != nil {
			slog.Error("error occurred when running timer function", "error", err.Error())
		}
	}
}
