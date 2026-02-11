package models

// JITOperationRequest - request body for JIT admin operations
type JITOperationRequest struct {
	Action    string `json:"action"` // enable, disable, request, approve, deny
	RequestID string `json:"request_id,omitempty"`
	GrantID   string `json:"grant_id,omitempty"`
	Reason    string `json:"reason,omitempty"`
	ExpiresAt int64  `json:"expires_at,omitempty"` // Unix epoch timestamp (seconds) for when access should expire
}

// JITAccessRequest - request body for user JIT access request
type JITAccessRequest struct {
	NetworkID string `json:"network_id"` // Network identifier
	Reason    string `json:"reason"`     // Reason for access request (required)
}

// UserJITNetworkStatus represents JIT status for a network from user's perspective
type UserJITNetworkStatus struct {
	NetworkID      string `json:"network_id"`
	NetworkName    string `json:"network_name,omitempty"`
	JITEnabled     bool   `json:"jit_enabled"`
	HasAccess      bool   `json:"has_access"`
	Grant          any    `json:"grant,omitempty"`   // schema.JITGrant
	Request        any    `json:"request,omitempty"` // schema.JITRequest
	PendingRequest bool   `json:"pending_request"`
}
