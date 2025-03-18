package utils

import (
	"log/slog"
	"runtime"
	"strings"
	"time"
)

// RetryStrategy specifies a strategy to retry an operation after waiting a while,
// with hooks for successful and unsuccessful (>=max) tries.
type RetryStrategy struct {
	Wait             func(time.Duration)
	WaitTime         time.Duration
	WaitTimeIncrease time.Duration
	MaxTries         int
	Try              func() error
	OnMaxTries       func()
	OnSuccess        func()
}

// DoStrategy does the retry strategy specified in the struct, waiting before retrying an operator,
// up to a max number of tries, and if executes a success "finalizer" operation if a retry is successful
func (rs RetryStrategy) DoStrategy() {
	err := rs.Try()
	if err == nil {
		rs.OnSuccess()
		return
	}

	tries := 1
	for {
		if tries >= rs.MaxTries {
			rs.OnMaxTries()
			return
		}
		rs.Wait(rs.WaitTime)
		if err := rs.Try(); err != nil {
			tries++                            // we tried, increase count
			rs.WaitTime += rs.WaitTimeIncrease // for the next time, sleep more
			continue                           // retry
		}
		rs.OnSuccess()
		return
	}
}

func TraceCaller() {
	// Skip 1 frame to get the caller of this function
	pc, file, line, ok := runtime.Caller(2)
	if !ok {
		slog.Debug("Unable to get caller information")
		return
	}

	// Get function name from the program counter (pc)
	funcName := runtime.FuncForPC(pc).Name()

	// Print trace details
	slog.Debug("Called from function: %s\n", "func-name", funcName)
	slog.Debug("File: %s, Line: %d\n", "file", file, "line-no", line)
}

// NoEmptyStringToCsv takes a bunch of strings, filters out empty ones and returns a csv version of the string
func NoEmptyStringToCsv(strs ...string) string {
	var sb strings.Builder
	for _, str := range strs {
		trimmedStr := strings.TrimSpace(str)
		if trimmedStr != "" && trimmedStr != "<nil>" {
			if sb.Len() > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(str)
		}
	}
	return sb.String()
}
