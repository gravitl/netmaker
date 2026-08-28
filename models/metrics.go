package models

import "github.com/gravitl/netmaker/schema"

type Metrics = schema.Metrics
type Metric = schema.Metric

// IDandAddr - struct to hold ID and primary Address
type IDandAddr struct {
	ID               string `json:"id" bson:"id" yaml:"id"`
	HostID           string `json:"host_id"`
	Address          string `json:"address" bson:"address" yaml:"address"`
	Address4         string `json:"address4"`
	Address6         string `json:"address6"`
	Name             string `json:"name" bson:"name" yaml:"name"`
	IsServer         string `json:"isserver" bson:"isserver" yaml:"isserver" validate:"checkyesorno"`
	Network          string `json:"network" bson:"network" yaml:"network" validate:"network"`
	ListenPort       int    `json:"listen_port" yaml:"listen_port"`
	IsExtClient      bool   `json:"is_extclient"`
	UserName         string `json:"username"`
	TcpProxyEndpoint string `json:"tcp_proxy_endpoint,omitempty"` // wss://host:port/uplink/v1 when peer is a TCP-enabled gateway
	TcpProxyCertFingerprint string `json:"tcp_proxy_cert_fingerprint,omitempty"`
}

// HostInfoMap - map of host public keys to host networking info
type HostInfoMap map[string]HostNetworkInfo

// HostNetworkInfo - holds info related to host networking (used for client side peer calculations)
type HostNetworkInfo struct {
	Interfaces         []schema.Iface `json:"interfaces" yaml:"interfaces"`
	ListenPort         int            `json:"listen_port" yaml:"listen_port"`
	IsStaticPort       bool           `json:"is_static_port"`
	IsStatic           bool           `json:"is_static"`
	Version            string         `json:"version"`
	TcpProxyEnabled    bool           `json:"tcp_proxy_enabled,omitempty"`
	TcpProxyListenPort int            `json:"tcp_proxy_listen_port,omitempty"`
	TcpProxyTLSMode    string         `json:"tcp_proxy_tls_mode,omitempty"`
	TcpProxyListenAddr     string         `json:"tcp_proxy_listen_addr,omitempty"`
	TcpProxyPublicHostname string         `json:"tcp_proxy_public_hostname,omitempty"`
	TcpProxyCertFingerprint string    `json:"tcp_proxy_cert_fingerprint,omitempty"`
}

// PeerMap - peer map for ids and addresses in metrics
type PeerMap map[string]IDandAddr

// MetricsMap - map for holding multiple metrics in memory
type MetricsMap map[string]Metrics

// NetworkMetrics - metrics model for all nodes in a network
type NetworkMetrics struct {
	Nodes MetricsMap `json:"nodes" bson:"nodes" yaml:"nodes"`
}
