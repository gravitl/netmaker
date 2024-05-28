package models

import (
	"time"
)

// Metrics - metrics struct
type Metrics struct {
	Network      string            `json:"network" bson:"network" yaml:"network"`
	NodeID       string            `json:"node_id" bson:"node_id" yaml:"node_id"`
	NodeName     string            `json:"node_name" bson:"node_name" yaml:"node_name"`
	Connectivity map[string]Metric `json:"connectivity" bson:"connectivity" yaml:"connectivity"`
}

// Metric - holds a metric for data between nodes
type Metric struct {
	NodeName      string        `json:"node_name" bson:"node_name" yaml:"node_name"`
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
	ID          string `json:"id" bson:"id" yaml:"id"`
	HostID      string `json:"host_id"`
	Address     string `json:"address" bson:"address" yaml:"address"`
	Name        string `json:"name" bson:"name" yaml:"name"`
	IsServer    string `json:"isserver" bson:"isserver" yaml:"isserver" validate:"checkyesorno"`
	Network     string `json:"network" bson:"network" yaml:"network" validate:"network"`
	ListenPort  int    `json:"listen_port" yaml:"listen_port"`
	IsExtClient bool   `json:"is_extclient"`
}

// HostInfoMap - map of host public keys to host networking info
type HostInfoMap map[string]HostNetworkInfo

// HostNetworkInfo - holds info related to host networking (used for client side peer calculations)
type HostNetworkInfo struct {
	Interfaces       []Iface `json:"interfaces" yaml:"interfaces"`
	ListenPort       int     `json:"listen_port" yaml:"listen_port"`
	IsStaticPort     bool    `json:"is_static_port"`
	IsStaticEndpoint bool    `json:"is_static_endpoint"`
}

// PeerMap - peer map for ids and addresses in metrics
type PeerMap map[string]IDandAddr

// MetricsMap - map for holding multiple metrics in memory
type MetricsMap map[string]Metrics

// NetworkMetrics - metrics model for all nodes in a network
type NetworkMetrics struct {
	Nodes MetricsMap `json:"nodes" bson:"nodes" yaml:"nodes"`
}
