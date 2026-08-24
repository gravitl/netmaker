package models

// DeviceNetwork describes a network from the device (desktop/netclient) API perspective.
type DeviceNetwork struct {
	NetworkID   string `json:"network_id"`
	DisplayName string `json:"display_name,omitempty"`
	Joined      bool   `json:"joined"`
	Connected   bool   `json:"connected"`
	Pending     bool   `json:"pending"`
	Status      string `json:"status"` // available | joined | pending | blocked | jit_required | approval_required

	ApprovalRequired    bool   `json:"approval_required"`
	ApprovalRequestedAt *int64 `json:"approval_requested_at,omitempty"`

	JITEnabled        bool   `json:"jit_enabled"`
	JITAppliesToUser  bool   `json:"jit_applies_to_user"`
	HasJITAccess      bool   `json:"has_jit_access"`
	JITPendingRequest bool   `json:"jit_pending_request"`
	JITGrant          any    `json:"jit_grant,omitempty"`
	JITRequest        any    `json:"jit_request,omitempty"`
	JITExpiresAt      *int64 `json:"jit_expires_at,omitempty"`
}

// DeviceJoinResult is returned from the device join API.
type DeviceJoinResult struct {
	Status string `json:"status"` // joined | pending
}

const (
	DeviceJoinStatusJoined  = "joined"
	DeviceJoinStatusPending = "pending"
)

const (
	DeviceNetworkStatusAvailable        = "available"
	DeviceNetworkStatusJoined           = "joined"
	DeviceNetworkStatusPending          = "pending"
	DeviceNetworkStatusBlocked          = "blocked"
	DeviceNetworkStatusJITRequired      = "jit_required"
	DeviceNetworkStatusApprovalRequired = "approval_required"
)

// DeviceExitNode describes an internet egress exit node available to a desktop device.
type DeviceExitNode struct {
	EgressID           string `json:"egress_id"`
	Name               string `json:"name"`
	Description        string `json:"description,omitempty"`
	Network            string `json:"network"`
	RoutingNodeID      string `json:"routing_node_id,omitempty"`
	RoutingHostName    string `json:"routing_host_name,omitempty"`
	// CountryCode is the ISO 3166-1 alpha-2 code of the routing host (for flags).
	CountryCode string `json:"country_code,omitempty"`
	// Location is "lat,lon" of the routing host when known.
	Location string `json:"location,omitempty"`
	// TcpProxyEnabled is true when the routing node (or its host) accepts TCP uplinks.
	TcpProxyEnabled    bool `json:"tcp_proxy_enabled"`
	TcpProxyListenPort int  `json:"tcp_proxy_listen_port,omitempty"`
	Selected           bool `json:"selected"`
	Status             bool `json:"status"`
}

// DeviceExitNodeSelectionReq selects or clears the exit node for a device on a network.
type DeviceExitNodeSelectionReq struct {
	EgressID     string `json:"egress_id"`
	UseTcpUplink bool   `json:"use_tcp_uplink"`
}

// NodeExitNodeSelectionReq selects or clears the exit node for a node (admin API).
type NodeExitNodeSelectionReq struct {
	EgressID     string `json:"egress_id"`
	UseTcpUplink bool   `json:"use_tcp_uplink"`
}
