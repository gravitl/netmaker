package logic

import (
	"context"
	"fmt"
	"github.com/gravitl/netmaker/logger"
	"golang.org/x/exp/slog"
	"sync"
	"time"

	"github.com/gravitl/netmaker/models"
)

// == Constants ==

// How long to wait before sending telemetry to server (24 hours)
const timer_hours_between_runs = 24

// HookManagerCh - channel to add any new hooks
var HookManagerCh = make(chan models.HookDetails, 2)

// == Public ==

// TimerCheckpoint - Checks if 24 hours has passed since telemetry was last sent. If so, sends telemetry data to posthog
func TimerCheckpoint() error {
	// get the telemetry record in the DB, which contains a timestamp
	telRecord, err := fetchTelemetryRecord()
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

// StartHookManager - listens on `HookManagerCh` to run any hook
func StartHookManager(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			slog.Error("## Stopping Hook Manager")
			return
		case newhook := <-HookManagerCh:
			wg.Add(1)
			go addHookWithInterval(ctx, wg, newhook.Hook, newhook.Interval)
		}
	}
}

func addHookWithInterval(ctx context.Context, wg *sync.WaitGroup, hook func() error, interval time.Duration) {
	defer wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := hook(); err != nil {
				slog.Error(err.Error())
			}
		}
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
