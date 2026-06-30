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

// DeviceJITAccessRequest is the body for requesting JIT access via the device API.
type DeviceJITAccessRequest struct {
	Reason string `json:"reason"`
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
