package metrics

import (
	"sync"
)

/*
1. Create metrics packet--> packet with identifier to track latency, errors.

*/

type Metric struct {
	LastRecordedLatency int64
	ConnectionStatus    bool
	TrafficSent         uint64
	TrafficRecieved     uint64
}

var MetricsMapLock *sync.RWMutex

var MetricsMap = make(map[string]Metric)
