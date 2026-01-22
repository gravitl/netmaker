package logic

import (
	"errors"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// JITStatusResponse - response for JIT status check
type JITStatusResponse struct {
	HasAccess      bool               `json:"has_access"`
	Grant          *schema.JITGrant   `json:"grant,omitempty"`
	Request        *schema.JITRequest `json:"request,omitempty"`
	PendingRequest bool               `json:"pending_request"`
}

// Pro logic functions (to avoid import cycles)
// These are set by pro/initialize.go when Pro is enabled
var EnableJITOnNetworkFunc func(string) error
var DisableJITOnNetworkFunc func(string) error
var ApproveJITRequestFunc func(string, int, string) (*schema.JITGrant, error)
var DenyJITRequestFunc func(string, string) error
var CheckJITAccessFunc func(string, string) (bool, *schema.JITGrant, error)
var GetNetworkJITRequestsFunc func(string, string) ([]interface{}, error)
var GetUserJITStatusFunc func(string, string) (*JITStatusResponse, error)
var GetUserJITNetworksStatusFunc func([]models.Network, string) ([]interface{}, error)
var DisconnectExtClientsFromNetworkFunc func(string) error
var GetNetworkAdminsFunc func(string) ([]models.User, error)

// ExpireJITGrants - expires grants that have passed their expiration time
// This is a variable that gets set by pro/initialize.go
var ExpireJITGrants func() error

// EnableJITOnNetwork - enables JIT on a network and disconnects existing ext clients
func EnableJITOnNetwork(networkID string) error {
	if EnableJITOnNetworkFunc == nil {
		return errors.New("JIT feature is not available")
	}
	return EnableJITOnNetworkFunc(networkID)
}

// DisableJITOnNetwork - disables JIT on a network
func DisableJITOnNetwork(networkID string) error {
	if DisableJITOnNetworkFunc == nil {
		return errors.New("JIT feature is not available")
	}
	return DisableJITOnNetworkFunc(networkID)
}

// ApproveJITRequest - approves a JIT request and creates a grant
func ApproveJITRequest(requestID string, durationHours int, approvedBy string) (*schema.JITGrant, error) {
	if ApproveJITRequestFunc == nil {
		return nil, errors.New("JIT feature is not available")
	}
	return ApproveJITRequestFunc(requestID, durationHours, approvedBy)
}

// DenyJITRequest - denies a JIT request
func DenyJITRequest(requestID string, deniedBy string) error {
	if DenyJITRequestFunc == nil {
		return errors.New("JIT feature is not available")
	}
	return DenyJITRequestFunc(requestID, deniedBy)
}

// CheckJITAccess - checks if a user has active JIT access for a network
func CheckJITAccess(networkID, userID string) (bool, *schema.JITGrant, error) {
	if CheckJITAccessFunc == nil {
		// Feature not available, allow access (backward compatibility)
		return true, nil, nil
	}
	return CheckJITAccessFunc(networkID, userID)
}

// GetNetworkJITRequests - gets JIT requests for a network, optionally filtered by status
// statusFilter can be: "pending", "approved", "denied", "expired", or "" for all
func GetNetworkJITRequests(networkID string, statusFilter string) ([]interface{}, error) {
	if GetNetworkJITRequestsFunc == nil {
		return nil, errors.New("JIT feature is not available")
	}
	return GetNetworkJITRequestsFunc(networkID, statusFilter)
}

// GetUserJITStatus - gets JIT status for a user on a network
func GetUserJITStatus(networkID, userID string) (*JITStatusResponse, error) {
	if GetUserJITStatusFunc == nil {
		return nil, errors.New("JIT feature is not available")
	}
	return GetUserJITStatusFunc(networkID, userID)
}

func expireJITGrants() error {
	if ExpireJITGrants == nil {
		return nil // No-op if feature not available
	}
	return ExpireJITGrants()
}

// DisconnectExtClientsFromNetwork - disconnects all ext clients from a network
func DisconnectExtClientsFromNetwork(networkID string) error {
	if DisconnectExtClientsFromNetworkFunc == nil {
		return errors.New("JIT feature is not available")
	}
	return DisconnectExtClientsFromNetworkFunc(networkID)
}

// GetNetworkAdmins - gets all network admins for a network
func GetNetworkAdmins(networkID string) ([]models.User, error) {
	if GetNetworkAdminsFunc == nil {
		return nil, errors.New("JIT feature is not available")
	}
	return GetNetworkAdminsFunc(networkID)
}
