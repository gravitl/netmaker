package ncutils

import (
	"fmt"
	"time"
)

// BackOff - back off any function while there is an error
func BackOff(isExponential bool, maxTime int, f interface{}) (interface{}, error) {
	// maxTime seconds
	startTime := time.Now()
	sleepTime := time.Second
	for time.Now().Before(startTime.Add(time.Second * time.Duration(maxTime))) {
		if result, err := f.(func() (interface{}, error))(); err == nil {
			return result, nil
		}
		time.Sleep(sleepTime)
		if isExponential {
			sleepTime = sleepTime << 1
		}
		PrintLog("retrying...", 1)
	}
	return nil, fmt.Errorf("could not find result")
}
