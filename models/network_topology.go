package models

import "github.com/gravitl/netmaker/schema"

const (
	// NetworkServerRoleGateway covers mesh gateways, ingress gateways, relays,
	// and auto-relays. Those are not separate roles.
	NetworkServerRoleGateway = "gateway"

	// NetworkEgressRoleEgress is a routing node of a cidr, domain, or app egress.
	NetworkEgressRoleEgress = "egress"
	// NetworkEgressRoleInternetGateway is a routing node of an internet egress (exit node).
	NetworkEgressRoleInternetGateway = "internet_gateway"
)

// NetworkConnectionType is how the clicked node reaches a peer.
type NetworkConnectionType string

const (
	NetworkConnectionP2P     = NetworkConnectionType("p2p")
	NetworkConnectionRelayed = NetworkConnectionType("relayed")
	NetworkConnectionExit    = NetworkConnectionType("exit")
)

// NetworkTopology is the graph of users, their devices, servers, and extclients.
// Peer lines are loaded separately when a vertex is selected.
type NetworkTopology struct {
	Network     string                `json:"network"`
	GeneratedAt int64                 `json:"generated_at"` // unix seconds
	Users       []NetworkTopologyUser `json:"users"`
	Servers     []NetworkTopologyNode `json:"servers"`
	ExtClients  []NetworkTopologyNode `json:"extclients"`
}

// NetworkTopologyUser groups a user's devices.
type NetworkTopologyUser struct {
	Username string                `json:"username"`
	Devices  []NetworkTopologyNode `json:"devices"`
}

// NetworkTopologyNode is a device, server, or extclient vertex.
type NetworkTopologyNode struct {
	ID        string            `json:"id"`
	Kind      NetworkNodeKind   `json:"kind"` // node | user | extclient
	Name      string            `json:"name"`
	HostID    string            `json:"host_id,omitempty"`
	OwnerID   string            `json:"owner_id,omitempty"` // username for user devices
	Address   string            `json:"address,omitempty"`
	Status    schema.NodeStatus `json:"status"`
	Connected bool              `json:"connected"`

	GatewayNodeID         string `json:"gateway_node_id,omitempty"`
	InternetGatewayNodeID string `json:"internet_gateway_node_id,omitempty"`

	// Roles is set on servers. The only value is "gateway".
	Roles []string `json:"roles,omitempty"`
	// Egresses are the active egresses this server routes. The role is derived
	// from the egress: internet_gateway for an internet egress, egress otherwise.
	Egresses []NetworkTopologyEgress `json:"egresses,omitempty"`
}

// NetworkTopologyEgress is an egress a server routes, and the role that gives the server.
type NetworkTopologyEgress struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // cidr | domain | app | internet
	Role string `json:"role"` // egress | internet_gateway
}

// NodeConnections is the clicked node's ACL-allowed peer lines.
type NodeConnections struct {
	NodeID      string              `json:"node_id"`
	Connections []NetworkConnection `json:"connections"`
}

// NetworkConnection is one peer line from the clicked node.
type NetworkConnection struct {
	PeerID    string                `json:"peer_id"`
	Name      string                `json:"name"`
	Kind      NetworkNodeKind       `json:"kind"`
	Type      NetworkConnectionType `json:"type"` // p2p | relayed | exit
	Via       *NetworkNodeRef       `json:"via,omitempty"`
	Connected bool                  `json:"connected"`
	LatencyMs int64                 `json:"latency_ms"`
	PercentUp float64               `json:"percent_up"`
}
