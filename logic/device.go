package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/gorm"
)

const DeviceHostIDHeader = "X-Host-ID"

var (
	// EnrichDeviceNetworksWithJIT adds JIT fields to device network responses (Pro).
	EnrichDeviceNetworksWithJIT = func(_ *schema.User, networks []models.DeviceNetwork) []models.DeviceNetwork {
		return networks
	}
	// RequestDeviceJITAccess handles JIT access requests from the device API (Pro).
	RequestDeviceJITAccess = func(_ *schema.User, _ string, _ string) (any, error) {
		return nil, errors.New("JIT feature is not enabled")
	}
	// PublishHostRegistrationUpdates notifies peers after host network join (wired from mq).
	PublishHostRegistrationUpdates = func(_ *schema.Host) error { return nil }
	// RequestHostPullUpdate asks a host to pull config (wired from mq).
	RequestHostPullUpdate = func(_ *schema.Host) error { return nil }
	// JoinHostToNetworks adds a host to networks (wired from auth).
	JoinHostToNetworks = func(_ models.EnrollmentKey, _ *schema.Host, _ string) {}
)

// EnsureHostOwner sets OwnerUsername on the host when unset.
func EnsureHostOwner(host *schema.Host, username string) {
	if host == nil || username == "" || host.OwnerUsername != "" {
		return
	}
	host.OwnerUsername = username
	_ = UpsertHost(host)
}

// VerifyDeviceHostAccess ensures the user may operate on the given host ID.
func VerifyDeviceHostAccess(ctx context.Context, username, hostIDStr string) (*schema.Host, error) {
	if username == "" || hostIDStr == "" {
		return nil, errors.New("user and host id are required")
	}
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid host id: %w", err)
	}
	host := &schema.Host{ID: hostID}
	if err := host.Get(db.WithContext(ctx)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("host not found")
		}
		return nil, err
	}
	if host.OwnerUsername == "" {
		EnsureHostOwner(host, username)
		return host, nil
	}
	if host.OwnerUsername != username {
		return nil, errors.New("host does not belong to user")
	}
	return host, nil
}

// GetDeviceNetworks returns networks accessible to the user with join/connection state for the host.
func GetDeviceNetworks(ctx context.Context, user *schema.User, host *schema.Host) ([]models.DeviceNetwork, error) {
	allNetworks, err := (&schema.Network{}).ListAll(ctx)
	if err != nil {
		return nil, err
	}
	accessible := FilterNetworksByRole(allNetworks, user)
	hostID := ""
	if host != nil {
		hostID = host.ID.String()
	}

	result := make([]models.DeviceNetwork, 0, len(accessible))
	for _, network := range accessible {
		dn := models.DeviceNetwork{
			NetworkID:   network.Name,
			DisplayName: network.Name,
			Status:      models.DeviceNetworkStatusAvailable,
			HasJITAccess: true,
		}
		if host != nil {
			if pending, _ := isHostPendingOnNetwork(ctx, hostID, network.Name); pending {
				dn.Pending = true
				dn.Status = models.DeviceNetworkStatusPending
			} else if node, err := getHostNodeOnNetwork(ctx, host, network.Name); err == nil {
				dn.Joined = true
				dn.Connected = node.Connected
				if node.Connected {
					dn.Status = models.DeviceNetworkStatusJoined
				} else {
					dn.Status = models.DeviceNetworkStatusAvailable
				}
			}
		}
		result = append(result, dn)
	}
	return EnrichDeviceNetworksWithJIT(user, result), nil
}

func isHostPendingOnNetwork(ctx context.Context, hostID, network string) (bool, error) {
	p := &schema.PendingHost{HostID: hostID, Network: network}
	err := p.CheckIfPendingHostExists(ctx)
	return err == nil, nil
}

func getHostNodeOnNetwork(ctx context.Context, host *schema.Host, network string) (*schema.Node, error) {
	net := &schema.Network{Name: network}
	if err := net.Get(ctx); err != nil {
		return nil, err
	}
	node := &schema.Node{
		HostID:    host.ID.String(),
		NetworkID: net.ID,
	}
	if err := node.GetByHostAndNetwork(ctx); err != nil {
		return nil, err
	}
	return node, nil
}

// JoinDeviceNetwork adds the host to a network on behalf of the user.
func JoinDeviceNetwork(ctx context.Context, user *schema.User, host *schema.Host, networkID string) error {
	if !IsUserAllowedToJoinNetwork(user.Username, networkID) {
		return errors.New("user does not have access to network")
	}
	hasAccess, _, err := CheckJITAccess(networkID, user.Username)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("JIT access required: please request access from network admin")
	}

	network := &schema.Network{Name: networkID}
	if err := network.Get(ctx); err != nil {
		return fmt.Errorf("network not found: %w", err)
	}
	if DoesHostExistInTheNetworkAlready(host, network) {
		return nil
	}

	violations, _ := CheckPostureViolationsForHost(host, nil, schema.NetworkID(networkID), true)
	if len(violations) > 0 {
		return errors.New("access blocked: this device doesn't meet security requirements")
	}

	featureFlags := GetFeatureFlags()
	if featureFlags.EnableDeviceApproval && !network.AutoJoin {
		p := &schema.PendingHost{HostID: host.ID.String(), Network: networkID}
		if err := p.CheckIfPendingHostExists(ctx); err == nil {
			return errors.New("host approval pending for network")
		}
		keyB, _ := json.Marshal(models.EnrollmentKey{Networks: []string{networkID}})
		pending := schema.PendingHost{
			ID:            uuid.NewString(),
			HostID:        host.ID.String(),
			Hostname:      host.Name,
			Network:       networkID,
			PublicKey:     host.PublicKey.String(),
			OS:            host.OS,
			Location:      host.Location,
			Version:       host.Version,
			EnrollmentKey: keyB,
			RequestedAt:   time.Now().UTC(),
		}
		if err := pending.Create(ctx); err != nil {
			return err
		}
		return errors.New("host approval required for network")
	}

	JoinHostToNetworks(models.EnrollmentKey{Networks: []string{networkID}}, host, user.Username)
	return nil
}

// LeaveDeviceNetwork removes the host from a network.
func LeaveDeviceNetwork(ctx context.Context, user *schema.User, host *schema.Host, networkID string) error {
	if !IsUserAllowedToJoinNetwork(user.Username, networkID) {
		return errors.New("user does not have access to network")
	}
	nodeSchema, err := getHostNodeOnNetwork(ctx, host, networkID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	node, err := GetNodeByID(nodeSchema.ID)
	if err != nil {
		return err
	}
	return DeleteNode(&node, true)
}

// SyncDevice requests the host to pull latest config via MQ.
func SyncDevice(host *schema.Host) error {
	if host == nil {
		return errors.New("host is required")
	}
	return RequestHostPullUpdate(host)
}
