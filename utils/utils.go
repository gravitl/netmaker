package utils

import "time"

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
