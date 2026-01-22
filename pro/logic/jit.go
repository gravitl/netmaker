package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"golang.org/x/exp/slog"

	"github.com/gravitl/netmaker/logic"
)

// Use JITStatusResponse from logic package to avoid type mismatch
type JITStatusResponse = logic.JITStatusResponse

// Email notification functions (set by pro/controllers to avoid import cycles)
var NotifyNetworkAdminsOfJITRequestFunc func(*schema.JITRequest, models.Network) error
var NotifyUserOfJITApprovalFunc func(*schema.JITGrant, *schema.JITRequest, models.Network) error

// EnableJITOnNetwork - enables JIT on a network and disconnects existing ext clients
func EnableJITOnNetwork(networkID string) error {
	// Check if JIT feature is enabled
	featureFlags := GetFeatureFlags()
	if !featureFlags.EnableJIT {
		return errors.New("JIT feature is not enabled")
	}

	network, err := logic.GetNetwork(networkID)
	if err != nil {
		return fmt.Errorf("failed to get network: %w", err)
	}

	network.JITEnabled = "yes"
	network.SetNetworkLastModified()

	if err := logic.SaveNetwork(&network); err != nil {
		return fmt.Errorf("failed to save network: %w", err)
	}

	// Disconnect all ext clients from this network
	if err := DisconnectExtClientsFromNetwork(networkID); err != nil {
		logger.Log(0, "failed to disconnect ext clients when enabling JIT:", err.Error())
		// Don't fail the operation, just log
	}

	return nil
}

// DisableJITOnNetwork - disables JIT on a network
func DisableJITOnNetwork(networkID string) error {
	network, err := logic.GetNetwork(networkID)
	if err != nil {
		return fmt.Errorf("failed to get network: %w", err)
	}

	network.JITEnabled = "no"
	network.SetNetworkLastModified()

	return logic.SaveNetwork(&network)
}

// CreateJITRequest - creates a new JIT access request
func CreateJITRequest(networkID, userName, reason string) (*schema.JITRequest, error) {
	// Check if JIT feature is enabled
	featureFlags := GetFeatureFlags()
	if !featureFlags.EnableJIT {
		return nil, errors.New("JIT feature is not enabled")
	}

	ctx := db.WithContext(context.Background())

	// Check if network exists and has JIT enabled
	network, err := logic.GetNetwork(networkID)
	if err != nil {
		return nil, fmt.Errorf("network not found: %w", err)
	}

	if network.JITEnabled != "yes" {
		return nil, errors.New("JIT is not enabled on this network")
	}

	// Check if user already has an active grant
	hasAccess, _, err := CheckJITAccess(networkID, userName)
	if err == nil && hasAccess {
		return nil, errors.New("user already has active access grant")
	}

	// Check if there's already a pending request
	request := schema.JITRequest{
		NetworkID: networkID,
		UserID:    userName,
	}
	pendingRequests, err := request.ListPendingByNetwork(ctx)
	if err == nil {
		for _, req := range pendingRequests {
			if req.UserID == userName {
				return nil, errors.New("user already has a pending request")
			}
		}
	}

	// Create new request
	newRequest := schema.JITRequest{
		ID:          uuid.New().String(),
		NetworkID:   networkID,
		UserID:      userName,
		UserName:    userName,
		Reason:      reason,
		Status:      "pending",
		RequestedAt: time.Now().UTC(),
	}

	if err := newRequest.Create(ctx); err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Send email notifications to network admins
	go func() {
		if NotifyNetworkAdminsOfJITRequestFunc != nil {
			if err := NotifyNetworkAdminsOfJITRequestFunc(&newRequest, network); err != nil {
				slog.Error("failed to send JIT request notifications", "error", err)
			}
		}
	}()

	return &newRequest, nil
}

// ApproveJITRequest - approves a JIT request and creates a grant
func ApproveJITRequest(requestID string, durationHours int, approvedBy string) (*schema.JITGrant, error) {
	ctx := db.WithContext(context.Background())

	// Get the request
	request := schema.JITRequest{ID: requestID}
	if err := request.Get(ctx); err != nil {
		return nil, fmt.Errorf("request not found: %w", err)
	}

	if request.Status != "pending" {
		return nil, errors.New("request is not pending")
	}

	// Update request status
	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(durationHours) * time.Hour)
	request.Status = "approved"
	request.ApprovedAt = now
	request.ApprovedBy = approvedBy
	request.DurationHours = durationHours
	request.ExpiresAt = expiresAt

	if err := request.Update(ctx); err != nil {
		return nil, fmt.Errorf("failed to update request: %w", err)
	}

	// Delete any existing grants for this user on this network
	if err := deactivateUserGrants(ctx, request.NetworkID, request.UserID); err != nil {
		slog.Warn("failed to delete existing grants", "error", err)
	}

	// Create new grant
	grant := schema.JITGrant{
		ID:        uuid.New().String(),
		NetworkID: request.NetworkID,
		UserID:    request.UserID,
		RequestID: request.ID,
		GrantedAt: now,
		ExpiresAt: expiresAt,
	}

	if err := grant.Create(ctx); err != nil {
		return nil, fmt.Errorf("failed to create grant: %w", err)
	}

	// Send approval email to user
	go func() {
		network, _ := logic.GetNetwork(request.NetworkID)
		if NotifyUserOfJITApprovalFunc != nil {
			if err := NotifyUserOfJITApprovalFunc(&grant, &request, network); err != nil {
				slog.Error("failed to send approval notification", "error", err)
			}
		}
	}()

	return &grant, nil
}

// DenyJITRequest - denies a JIT request
func DenyJITRequest(requestID string, deniedBy string) error {
	ctx := db.WithContext(context.Background())

	request := schema.JITRequest{ID: requestID}
	if err := request.Get(ctx); err != nil {
		return fmt.Errorf("request not found: %w", err)
	}

	if request.Status != "pending" {
		return errors.New("request is not pending")
	}

	now := time.Now().UTC()
	request.Status = "denied"
	request.ApprovedAt = now
	request.ApprovedBy = deniedBy

	return request.Update(ctx)
}

// CheckJITAccess - checks if a user has active JIT access for a network
func CheckJITAccess(networkID, userID string) (bool, *schema.JITGrant, error) {
	// Check if JIT feature is enabled
	featureFlags := GetFeatureFlags()
	if !featureFlags.EnableJIT {
		// Feature flag disabled, allow access (backward compatibility)
		return true, nil, nil
	}
	fmt.Println("====> Here1 Checking JIT access: ", userID)
	ctx := db.WithContext(context.Background())

	// Check if network has JIT enabled
	network, err := logic.GetNetwork(networkID)
	if err != nil {
		return false, nil, fmt.Errorf("network not found: %w", err)
	}

	if network.JITEnabled != "yes" {
		// JIT not enabled, allow access
		return true, nil, nil
	}
	fmt.Println("====> Here2 Checking JIT access: ", userID)
	// Check for active grant
	grant := schema.JITGrant{
		NetworkID: networkID,
		UserID:    userID,
	}

	activeGrant, err := grant.GetActiveByUserAndNetwork(ctx)
	if err != nil {
		// No active grant found
		return false, nil, nil
	}

	// Check if grant is expired
	if time.Now().UTC().After(activeGrant.ExpiresAt) {
		// Grant expired, delete it
		_ = activeGrant.Delete(ctx)
		return false, nil, nil
	}

	return true, activeGrant, nil
}

// JITRequestWithGrant - JIT request with grant ID for approved requests
type JITRequestWithGrant struct {
	schema.JITRequest
	GrantID string `json:"grant_id,omitempty"` // Grant ID if request is approved
}

// GetNetworkJITRequests - gets JIT requests for a network, optionally filtered by status
// statusFilter can be: "pending", "approved", "denied", "expired", or "" for all
func GetNetworkJITRequests(networkID string, statusFilter string) ([]JITRequestWithGrant, error) {
	ctx := db.WithContext(context.Background())

	request := schema.JITRequest{NetworkID: networkID}
	var requests []schema.JITRequest
	var err error

	// If no filter, return all requests
	if statusFilter == "" || statusFilter == "all" {
		requests, err = request.ListByNetwork(ctx)
		if err != nil {
			return nil, err
		}
	} else if statusFilter == "expired" {
		// Handle expired filter (approved requests that have expired)
		allRequests, err := request.ListByNetwork(ctx)
		if err != nil {
			return nil, err
		}

		now := time.Now().UTC()
		for _, req := range allRequests {
			// Include requests with status "expired" or "approved" requests that have passed expiration
			if req.Status == "expired" ||
				(req.Status == "approved" && !req.ExpiresAt.IsZero() && now.After(req.ExpiresAt)) {
				requests = append(requests, req)
			}
		}
	} else {
		// Filter by status: pending, approved, or denied
		requests, err = request.ListByStatusAndNetwork(ctx, statusFilter)
		if err != nil {
			return nil, err
		}
	}

	// Enrich requests with grant_id for approved requests
	result := make([]JITRequestWithGrant, 0, len(requests))
	for _, req := range requests {
		enriched := JITRequestWithGrant{
			JITRequest: req,
		}

		// If request is approved or expired, get the associated grant ID
		if req.Status == "approved" || req.Status == "expired" {
			grant := schema.JITGrant{RequestID: req.ID}
			if grantObj, err := grant.GetByRequestID(ctx); err == nil {
				enriched.GrantID = grantObj.ID
			}
		}

		result = append(result, enriched)
	}

	return result, nil
}

// GetUserJITStatus - gets JIT status for a user on a network
func GetUserJITStatus(networkID, userID string) (*JITStatusResponse, error) {
	ctx := db.WithContext(context.Background())

	response := &JITStatusResponse{}

	// Check for active grant
	hasAccess, grant, err := CheckJITAccess(networkID, userID)
	if err != nil {
		return nil, err
	}

	response.HasAccess = hasAccess
	response.Grant = grant

	// Check for pending request
	request := schema.JITRequest{
		NetworkID: networkID,
		UserID:    userID,
	}
	pendingRequests, err := request.ListPendingByNetwork(ctx)
	if err == nil {
		for _, req := range pendingRequests {
			if req.UserID == userID {
				response.PendingRequest = true
				response.Request = &req
				break
			}
		}
	}

	return response, nil
}

// UserJITNetworkStatus - represents JIT status for a network from user's perspective
type UserJITNetworkStatus struct {
	NetworkID      string             `json:"network_id"`
	NetworkName    string             `json:"network_name,omitempty"`
	JITEnabled     bool               `json:"jit_enabled"`
	HasAccess      bool               `json:"has_access"`
	Grant          *schema.JITGrant   `json:"grant,omitempty"`
	Request        *schema.JITRequest `json:"request,omitempty"`
	PendingRequest bool               `json:"pending_request"`
}

// GetUserJITNetworksStatus - gets JIT status for multiple networks for a user
func GetUserJITNetworksStatus(networks []models.Network, userID string) ([]UserJITNetworkStatus, error) {
	ctx := db.WithContext(context.Background())
	var result []UserJITNetworkStatus

	for _, network := range networks {
		status := UserJITNetworkStatus{
			NetworkID:      network.NetID,
			NetworkName:    network.NetID, // Can be enhanced with network display name if available
			JITEnabled:     network.JITEnabled == "yes",
			HasAccess:      false,
			PendingRequest: false,
		}

		// Only check JIT status if JIT is enabled on the network
		if status.JITEnabled {
			// Check for active grant
			hasAccess, grant, err := CheckJITAccess(network.NetID, userID)
			if err != nil {
				slog.Warn("failed to check JIT access", "network", network.NetID, "user", userID, "error", err)
				// Continue with default values
			} else {
				status.HasAccess = hasAccess
				status.Grant = grant
			}

			// Check for pending request
			request := schema.JITRequest{
				NetworkID: network.NetID,
				UserID:    userID,
			}
			pendingRequests, err := request.ListPendingByNetwork(ctx)
			if err == nil {
				for _, req := range pendingRequests {
					if req.UserID == userID {
						status.PendingRequest = true
						status.Request = &req
						break
					}
				}
			}
		}

		result = append(result, status)
	}

	return result, nil
}

// ExpireJITGrants - expires grants that have passed their expiration time
func ExpireJITGrants() error {
	ctx := db.WithContext(context.Background())

	grant := schema.JITGrant{}
	expiredGrants, err := grant.ListExpired(ctx)
	if err != nil {
		return fmt.Errorf("failed to list expired grants: %w", err)
	}

	for _, expiredGrant := range expiredGrants {
		// Update associated request status to "expired" before deleting grant
		if expiredGrant.RequestID != "" {
			request := schema.JITRequest{ID: expiredGrant.RequestID}
			if err := request.Get(ctx); err == nil {
				// Only update if request is currently approved
				if request.Status == "approved" {
					request.Status = "expired"
					if err := request.Update(ctx); err != nil {
						slog.Warn("failed to update request status when expiring grant",
							"grant_id", expiredGrant.ID, "request_id", expiredGrant.RequestID, "error", err)
						// Don't fail the operation, just log
					}
				}
			}
		}

		// Disconnect user's ext clients from the network
		if err := disconnectUserExtClients(expiredGrant.NetworkID, expiredGrant.UserID); err != nil {
			slog.Error("failed to disconnect ext clients for expired grant",
				"grant_id", expiredGrant.ID, "user_id", expiredGrant.UserID, "error", err)
		}

		// Delete the expired grant
		if err := expiredGrant.Delete(ctx); err != nil {
			slog.Error("failed to delete expired grant", "grant_id", expiredGrant.ID, "error", err)
			continue
		}

		logger.Log(1, fmt.Sprintf("Expired and deleted JIT grant %s for user %s on network %s",
			expiredGrant.ID, expiredGrant.UserID, expiredGrant.NetworkID))
	}

	return nil
}

// DisconnectExtClientsFromNetwork - disconnects all ext clients from a network
func DisconnectExtClientsFromNetwork(networkID string) error {
	extClients, err := logic.GetNetworkExtClients(networkID)
	if err != nil {
		return fmt.Errorf("failed to get ext clients: %w", err)
	}

	for _, client := range extClients {
		if err := logic.DeleteExtClient(client.Network, client.ClientID, false); err != nil {
			slog.Warn("failed to delete ext client when disabling JIT",
				"client_id", client.ClientID, "network", networkID, "error", err)
			continue
		}

		// DeleteExtClient handles MQ notifications internally
	}

	return nil
}

// GetNetworkAdmins - gets all network admins for a network
func GetNetworkAdmins(networkID string) ([]models.User, error) {
	var admins []models.User

	users, err := logic.GetUsersDB()
	if err != nil {
		return admins, fmt.Errorf("failed to get users: %w", err)
	}

	networkIDModel := models.NetworkID(networkID)
	allNetworksID := models.AllNetworks

	for _, user := range users {
		isAdmin := false

		// Check platform role (super admin or admin)
		if user.PlatformRoleID == models.SuperAdminRole || user.PlatformRoleID == models.AdminRole {
			isAdmin = true
		}

		// Check network-specific roles
		if roles, ok := user.NetworkRoles[networkIDModel]; ok {
			for roleID := range roles {
				if roleID == models.NetworkAdmin {
					isAdmin = true
					break
				}
			}
		}

		// Check all-networks role
		globalNetworksAdminRoleID := models.UserRoleID(fmt.Sprintf("global-%s", models.NetworkAdmin))
		if roles, ok := user.NetworkRoles[allNetworksID]; ok {
			for roleID := range roles {
				if roleID == models.NetworkAdmin || roleID == globalNetworksAdminRoleID {
					isAdmin = true
					break
				}
			}
		}

		// Check user groups
		for groupID := range user.UserGroups {
			groupData, err := database.FetchRecord(database.USER_GROUPS_TABLE_NAME, groupID.String())
			if err != nil {
				continue
			}

			var group models.UserGroup
			if err := json.Unmarshal([]byte(groupData), &group); err != nil {
				continue
			}

			// Check if group has network admin role for this network
			if roles, ok := group.NetworkRoles[networkIDModel]; ok {
				for roleID := range roles {
					if roleID == models.NetworkAdmin {
						isAdmin = true
						break
					}
				}
			}

			if roles, ok := group.NetworkRoles[allNetworksID]; ok {
				for roleID := range roles {
					if roleID == models.NetworkAdmin || roleID == globalNetworksAdminRoleID {
						isAdmin = true
						break
					}
				}
			}
		}

		if isAdmin {
			admins = append(admins, user)
		}
	}

	return admins, nil
}

// Helper functions

func deactivateUserGrants(ctx context.Context, networkID, userID string) error {
	return DeactivateUserGrantsOnNetwork(networkID, userID)
}

// DeactivateUserGrantsOnNetwork - deletes all active grants for a user on a network
func DeactivateUserGrantsOnNetwork(networkID, userID string) error {
	ctx := db.WithContext(context.Background())
	grant := schema.JITGrant{
		NetworkID: networkID,
		UserID:    userID,
	}
	grants, err := grant.ListByUserAndNetwork(ctx)
	if err != nil {
		return err
	}

	for _, g := range grants {
		// Only delete grants that haven't expired yet (active grants)
		if time.Now().UTC().Before(g.ExpiresAt) {
			if err := g.Delete(ctx); err != nil {
				return fmt.Errorf("failed to delete grant %s: %w", g.ID, err)
			}
		}
	}

	return nil
}

// DisconnectUserExtClientsFromNetwork - disconnects a specific user's ext clients from a network
func DisconnectUserExtClientsFromNetwork(networkID, userID string) error {
	return disconnectUserExtClients(networkID, userID)
}

func disconnectUserExtClients(networkID, userID string) error {
	extClients, err := logic.GetNetworkExtClients(networkID)
	if err != nil {
		return err
	}

	for _, client := range extClients {
		// Check if this ext client belongs to the user
		// Ext clients have OwnerID field that should match userID
		if client.OwnerID == userID {
			if err := logic.DeleteExtClient(client.Network, client.ClientID, false); err != nil {
				slog.Warn("failed to delete ext client", "client_id", client.ClientID, "error", err)
				continue
			}

			// DeleteExtClient handles MQ notifications internally
		}
	}

	return nil
}
