package logic

import (
	"fmt"
	"time"

	"github.com/gravitl/netmaker/logger"
)

// timeHooks - functions to run once a day, functions must take no parameters
var timeHooks = []interface{}{
	loggerDump,
	sendTelemetry,
}

func loggerDump() {
	logger.DumpFile(fmt.Sprintf("data/netmaker.log.%s", time.Now().Format(logger.TimeFormatDay)))
}

// TIMER_HOURS_BETWEEN_RUN - How long to wait before sending telemetry to server (24 hours)
const TIMER_HOURS_BETWEEN_RUN = 24

// AddHook - adds a hook function to run every 24hrs
func AddHook(ifaceToAdd interface{}) {
	timeHooks = append(timeHooks, ifaceToAdd)
}

// runHooks - runs the functions currently in the timeHooks data structure
func runHooks() {
	for _, hook := range timeHooks {
		hook.(func())()
	}
}

// TimerCheckpoint - Checks if 24 hours has passed since telemetry was last sent. If so, sends telemetry data to posthog
func TimerCheckpoint() error {

	// get the telemetry record in the DB, which contains a timestamp
	telRecord, err := fetchTelemetryRecord()
	if err != nil {
		return err
	}
	sendtime := time.Unix(telRecord.LastSend, 0).Add(time.Hour * time.Duration(TIMER_HOURS_BETWEEN_RUN))
	// can set to 2 minutes for testing
	// sendtime := time.Unix(telRecord.LastSend, 0).Add(time.Minute * 2)
	enoughTimeElapsed := time.Now().After(sendtime)
	// if more than 24 hours has elapsed, send telemetry to posthog
	if enoughTimeElapsed {
		// run any time hooks
		runHooks()
	}
	// set telemetry timestamp for server, restarts 24 hour cycle
	return setTelemetryTimestamp(telRecord.UUID)
}
