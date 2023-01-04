package metrics

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

/*
1. Create metrics packet--> packet with identifier to track latency, errors.

*/

type Metric struct {
	LastRecordedLatency uint64
	ConnectionStatus    bool
	TrafficSent         float64
	TrafficRecieved     float64
}

type MetricsPayload struct {
	MetricType MetricsUpdateType
	Value      interface{}
}

type MetricsUpdateType uint32

const (
	LatencyUpdate         MetricsUpdateType = 1
	TrafficSentUpdate     MetricsUpdateType = 2
	TrafficRecievedUpdate MetricsUpdateType = 3
)

var MetricsMapLock = &sync.RWMutex{}

var MetricsMap = make(map[string]Metric)

func init() {
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			PrintMetrics()
		}
	}()
}

func PrintMetrics() {

	data, err := json.MarshalIndent(MetricsMap, "", " ")
	if err != nil {
		return
	}
	os.WriteFile("/tmp/metrics.json", data, 0755)

}
