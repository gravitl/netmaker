package models

import "time"

// Metrics - metrics struct
type Metrics struct {
	Network       string            `json:"network" bson:"network" yaml:"network"`
	NodeID        string            `json:"node_id" bson:"node_id" yaml:"node_id"`
	NodeName      string            `json:"node_name" bson:"node_name" yaml:"node_name"`
	IsServer      string            `json:"isserver" bson:"isserver" yaml:"isserver" validate:"checkyesorno"`
	Connectivity  map[string]Metric `json:"connectivity" bson:"connectivity" yaml:"connectivity"`
	FailoverPeers map[string]string `json:"needsfailover" bson:"needsfailover" yaml:"needsfailover"`
}

// Metric - holds a metric for data between nodes
type Metric struct {
	NodeName      string        `json:"node_name" bson:"node_name" yaml:"node_name"`
	IsServer      string        `json:"isserver" bson:"isserver" yaml:"isserver" validate:"checkyesorno"`
	Uptime        int64         `json:"uptime" bson:"uptime" yaml:"uptime"`
	TotalTime     int64         `json:"totaltime" bson:"totaltime" yaml:"totaltime"`
	Latency       int64         `json:"latency" bson:"latency" yaml:"latency"`
	TotalReceived int64         `json:"totalreceived" bson:"totalreceived" yaml:"totalreceived"`
	TotalSent     int64         `json:"totalsent" bson:"totalsent" yaml:"totalsent"`
	ActualUptime  time.Duration `json:"actualuptime" bson:"actualuptime" yaml:"actualuptime"`
	PercentUp     float64       `json:"percentup" bson:"percentup" yaml:"percentup"`
	Connected     bool          `json:"connected" bson:"connected" yaml:"connected"`
}

// IDandAddr - struct to hold ID and primary Address
type IDandAddr struct {
	ID       string `json:"id" bson:"id" yaml:"id"`
	Address  string `json:"address" bson:"address" yaml:"address"`
	Name     string `json:"name" bson:"name" yaml:"name"`
	IsServer string `json:"isserver" bson:"isserver" yaml:"isserver" validate:"checkyesorno"`
}

// PeerMap - peer map for ids and addresses in metrics
type PeerMap map[string]IDandAddr

// MetricsMap - map for holding multiple metrics in memory
type MetricsMap map[string]Metrics

// NetworkMetrics - metrics model for all nodes in a network
type NetworkMetrics struct {
	Nodes MetricsMap `json:"nodes" bson:"nodes" yaml:"nodes"`
}
