package models

import "github.com/gravitl/netmaker/schema"

type Metrics = schema.Metrics
type Metric = schema.Metric

// IDandAddr - struct to hold ID and primary Address
type IDandAddr struct {
	ID          string `json:"id" bson:"id" yaml:"id"`
	HostID      string `json:"host_id"`
	Address     string `json:"address" bson:"address" yaml:"address"`
	Address4    string `json:"address4"`
	Address6    string `json:"address6"`
	Name        string `json:"name" bson:"name" yaml:"name"`
	IsServer    string `json:"isserver" bson:"isserver" yaml:"isserver" validate:"checkyesorno"`
	Network     string `json:"network" bson:"network" yaml:"network" validate:"network"`
	ListenPort  int    `json:"listen_port" yaml:"listen_port"`
	IsExtClient bool   `json:"is_extclient"`
	UserName    string `json:"username"`
}

// HostInfoMap - map of host public keys to host networking info
type HostInfoMap map[string]HostNetworkInfo

// HostNetworkInfo - holds info related to host networking (used for client side peer calculations)
type HostNetworkInfo struct {
	Interfaces   []schema.Iface `json:"interfaces" yaml:"interfaces"`
	ListenPort   int            `json:"listen_port" yaml:"listen_port"`
	IsStaticPort bool           `json:"is_static_port"`
	IsStatic     bool           `json:"is_static"`
	Version      string         `json:"version"`
}

// PeerMap - peer map for ids and addresses in metrics
type PeerMap map[string]IDandAddr

// MetricsMap - map for holding multiple metrics in memory
type MetricsMap map[string]Metrics

// NetworkMetrics - metrics model for all nodes in a network
type NetworkMetrics struct {
	Nodes MetricsMap `json:"nodes" bson:"nodes" yaml:"nodes"`
}
