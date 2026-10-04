package models

import "github.com/gravitl/netmaker/schema"

// NetworkNodeKind is what an entry in the network status is.
type NetworkNodeKind string

const (
	NetworkNodeKindNode      = NetworkNodeKind("node")
	NetworkNodeKindUser      = NetworkNodeKind("user") // user-registered device or user extclient
	NetworkNodeKindExtClient = NetworkNodeKind("extclient")
)

const (
	PeerConnectionDirect  = "direct"
	PeerConnectionRelayed = "relayed"
)

// NetworkStatus is a monitoring snapshot of every node and extclient in a network.
type NetworkStatus struct {
	Network     string               `json:"network"`
	GeneratedAt int64                `json:"generated_at"` // unix seconds
	Summary     NetworkStatusSummary `json:"summary"`
	Nodes       []NetworkNodeStatus  `json:"nodes"`
}

// NetworkStatusSummary counts the network's nodes and extclients by status.
type NetworkStatusSummary struct {
	Total        int `json:"total"`
	Online       int `json:"online"`
	Offline      int `json:"offline"`
	Warning      int `json:"warning"`
	Error        int `json:"error"`
	Unknown      int `json:"unknown"`
	Disconnected int `json:"disconnected"`
}

// NetworkNodeStatus is the status of a node or extclient in a network.
type NetworkNodeStatus struct {
	ID         string          `json:"id"`   // node ID, or client ID for extclients
	Kind       NetworkNodeKind `json:"kind"` // node | user | extclient
	Name       string          `json:"name"` // host name, or client ID for extclients
	HostID     string          `json:"host_id,omitempty"`
	MacAddress string          `json:"mac_address,omitempty"` // host MAC; RemoteAccessClientID for RAC clients
	Owner      string          `json:"owner,omitempty"`       // username for user devices and user extclients
	Address    string          `json:"address,omitempty"`     // overlay IPv4
	Address6   string          `json:"address6,omitempty"`    // overlay IPv6
	EndpointIP string          `json:"endpoint_ip,omitempty"` // public IP
	OS         string          `json:"os,omitempty"`
	Version    string          `json:"version,omitempty"`

	IsGateway         bool `json:"is_gateway"`          // gateway or relay
	IsInternetGateway bool `json:"is_internet_gateway"` // routes an active internet egress
	IsEgress          bool `json:"is_egress"`           // routes an active non-internet egress

	Status           schema.NodeStatus `json:"status"`
	Connected        bool              `json:"connected"`          // admin connect/enable toggle
	LastCheckIn      int64             `json:"last_check_in"`      // unix seconds, 0 = never
	MetricsUpdatedAt int64             `json:"metrics_updated_at"` // unix seconds, 0 = no metrics

	GatewayNodeID         string `json:"gateway_node_id,omitempty"`          // gateway the node or extclient connects through
	InternetGatewayNodeID string `json:"internet_gateway_node_id,omitempty"` // node currently used as internet gateway

	ConnectedPeers int                 `json:"connected_peers"`
	TotalPeers     int                 `json:"total_peers"`
	Peers          []NetworkPeerStatus `json:"peers,omitempty"`
}

// NetworkPeerStatus is a node's link to one peer, as reported in its metrics.
type NetworkPeerStatus struct {
	PeerID         string  `json:"peer_id"`
	Name           string  `json:"name"`
	Connected      bool    `json:"connected"`
	LatencyMs      int64   `json:"latency_ms"`
	ConnectionType string  `json:"connection_type"`         // direct | relayed
	RelayNodeID    string  `json:"relay_node_id,omitempty"` // set when relayed
	PercentUp      float64 `json:"percent_up"`
	BytesSent      int64   `json:"bytes_sent"`
	BytesReceived  int64   `json:"bytes_received"`
}
