package logic

import (
	"fmt"
	"time"

	"github.com/gravitl/netmaker/logger"
)

// == Constants ==

// How long to wait before sending telemetry to server (24 hours)
const timer_hours_between_runs = 24

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

// == private ==

// timeHooks - functions to run once a day, functions must take no parameters
var timeHooks = []interface{}{
	loggerDump,
	sendTelemetry,
}

func loggerDump() error {
	logger.DumpFile(fmt.Sprintf("data/netmaker.log.%s", time.Now().Format(logger.TimeFormatDay)))
	return nil
}

// runHooks - runs the functions currently in the timeHooks data structure
func runHooks() {
	for _, hook := range timeHooks {
		if err := hook.(func() error)(); err != nil {
			logger.Log(1, "error occurred when running timer function:", err.Error())
		}
	}
}
