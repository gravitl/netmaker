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

	// AutoSelectExitNode is set by admins. User devices must use an exit node;
	// the nearest allowed exit is chosen when they connect.
	AutoSelectExitNode bool `json:"auto_select_exit_node"`
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
	EgressID        string `json:"egress_id"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	Network         string `json:"network"`
	RoutingNodeID   string `json:"routing_node_id,omitempty"`
	RoutingHostName string `json:"routing_host_name,omitempty"`
	// Address / Address6 are the routing node's WireGuard overlay IPs. Clients
	// probe these on the metrics port for latency and reachability (same path
	// as mesh metrics collection), not the public AllowedEndpoints.
	Address  string `json:"address,omitempty"`
	Address6 string `json:"address6,omitempty"`
	// AllowedEndpoints are the routing host public IPs (EndpointIP, EndpointIPv6).
	AllowedEndpoints []string `json:"allowed_endpoints,omitempty"`
	// CountryCode is the ISO 3166-1 alpha-2 code of the routing host (for flags).
	CountryCode string `json:"country_code,omitempty"`
	// Location is "lat,lon" of the routing host when known.
	Location string `json:"location,omitempty"`
	// TcpProxyEnabled is true when the routing node (or its host) accepts TCP uplinks.
	TcpProxyEnabled    bool `json:"tcp_proxy_enabled"`
	TcpProxyListenPort int  `json:"tcp_proxy_listen_port,omitempty"`
	Selected           bool `json:"selected"`
	Status             bool `json:"status"`
	// LatencyMs is filled by netclient from overlay metrics-port probes; not set by the server.
	LatencyMs int64 `json:"latency_ms,omitempty"`
	// Nearest is filled by netclient for the closest exit; not set by the server.
	Nearest bool `json:"nearest,omitempty"`
}

// DeviceExitNodeSelectionReq selects or clears the exit node for a device on a network.
type DeviceExitNodeSelectionReq struct {
	EgressID     string `json:"egress_id"`
	UseTcpUplink bool   `json:"use_tcp_uplink"`
	// Force allows clearing the exit on networks that require auto_select_exit_node
	// so clients can clear-then-switch during auto failover without a 400.
	Force bool `json:"force,omitempty"`
}

// NodeExitNodeSelectionReq selects or clears the exit node for a node (admin API).
type NodeExitNodeSelectionReq struct {
	EgressID     string `json:"egress_id"`
	UseTcpUplink bool   `json:"use_tcp_uplink"`
}
