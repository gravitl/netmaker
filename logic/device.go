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
	"golang.org/x/exp/slog"
	"gorm.io/gorm"
)

const DeviceHostIDHeader = "X-Host-ID"

var (
	// EnrichDeviceNetworksWithJIT adds JIT fields to device network responses (Pro).
	EnrichDeviceNetworksWithJIT = func(_ *schema.User, _ []schema.Network, networks []models.DeviceNetwork) []models.DeviceNetwork {
		return networks
	}
	// PublishHostRegistrationUpdates notifies peers after host network join (wired from mq).
	PublishHostRegistrationUpdates = func(_ *schema.Host) error { return nil }
	// RequestHostPullUpdate asks a host to pull config (wired from mq).
	RequestHostPullUpdate = func(_ *schema.Host) error { return nil }
	// JoinHostToNetworks adds a host to networks (wired from auth).
	JoinHostToNetworks = func(_ models.EnrollmentKey, _ *schema.Host, _ string) {}
	// ProvisionDeviceHostMessaging creates broker credentials for a new device host (wired from mq).
	ProvisionDeviceHostMessaging = func(_ *schema.Host) error { return nil }
	// CleanupDeviceHostForOwnershipTransfer removes prior user network state before host re-bind (wired from mq).
	CleanupDeviceHostForOwnershipTransfer = DefaultCleanupDeviceHostForOwnershipTransfer
)

// DefaultCleanupDeviceHostForOwnershipTransfer removes pending joins and network nodes from a host.
func DefaultCleanupDeviceHostForOwnershipTransfer(ctx context.Context, host *schema.Host) error {
	if host == nil {
		return errors.New("host is required")
	}
	pending := &schema.PendingHost{HostID: host.ID.String()}
	if err := pending.DeleteAllPendingHosts(ctx); err != nil {
		return err
	}
	return DisassociateAllNodesFromHost(host.ID.String())
}

// TransferDeviceHostOwnership re-binds a shared desktop host to a new user, cleaning up the prior owner's network state.
func TransferDeviceHostOwnership(ctx context.Context, host *schema.Host, newOwner string) error {
	if host == nil || newOwner == "" {
		return errors.New("host and new owner are required")
	}
	if host.OwnerUsername == newOwner {
		return nil
	}
	previousOwner := host.OwnerUsername
	if err := CleanupDeviceHostForOwnershipTransfer(ctx, host); err != nil {
		return fmt.Errorf("failed to cleanup host for ownership transfer: %w", err)
	}
	host.OwnerUsername = newOwner
	if err := UpsertHost(host); err != nil {
		return err
	}
	logDeviceOwnershipTransfer(newOwner, host, previousOwner)
	slog.Info("transferred device host ownership",
		"host", host.ID.String(),
		"from", previousOwner,
		"to", newOwner,
	)
	return nil
}

func logDeviceOwnershipTransfer(newOwner string, host *schema.Host, previousOwner string) {
	if host == nil || newOwner == "" || previousOwner == "" || previousOwner == newOwner {
		return
	}
	LogEvent(&models.Event{
		Action: schema.TransferDeviceOwnership,
		Source: models.Subject{
			ID:   newOwner,
			Name: newOwner,
			Type: schema.UserSub,
		},
		TriggeredBy: newOwner,
		Target: models.Subject{
			ID:   host.ID.String(),
			Name: host.Name,
			Type: schema.DeviceSub,
		},
		Diff: models.Diff{
			Old: map[string]string{"owner_username": previousOwner},
			New: map[string]string{"owner_username": newOwner},
		},
		Origin: schema.ClientApp,
	})
}

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
	if len(accessible) == 0 && user != nil && len(user.UserGroups.Data()) > 0 {
		slog.Warn("device networks empty for user with groups",
			"username", user.Username,
			"role", user.PlatformRoleID,
			"groups", len(user.UserGroups.Data()),
			"total_networks", len(allNetworks),
		)
	}
	featureFlags := GetFeatureFlags()
	result := make([]models.DeviceNetwork, 0, len(accessible))
	for _, network := range accessible {
		dn := models.DeviceNetwork{
			NetworkID:    network.Name,
			DisplayName:  network.Name,
			Status:       models.DeviceNetworkStatusAvailable,
			HasJITAccess: true,
		}
		applyDeviceNetworkApprovalPolicy(network, user, featureFlags, &dn)
		if host != nil {
			applyDeviceNetworkHostState(ctx, host, network, user, featureFlags, &dn)
		}
		result = append(result, dn)
	}
	return EnrichDeviceNetworksWithJIT(user, accessible, result), nil
}

// deviceJoinRequiresApproval reports whether a user-owned device join should enter
// pending-host approval instead of joining immediately.
func deviceJoinRequiresApproval(network schema.Network, user *schema.User) bool {
	featureFlags := GetFeatureFlags()
	if !featureFlags.EnableDeviceApproval || network.AutoJoin {
		return false
	}
	if user != nil && IsNetworkAdmin(user, network.Name) {
		return false
	}
	// When JIT gates this user, admin approval happens via the JIT grant flow.
	if network.JITEnabled && UserSubjectToNetworkJIT(network.Name, user) {
		return false
	}
	return true
}

func applyDeviceNetworkApprovalPolicy(network schema.Network, user *schema.User, featureFlags models.FeatureFlags, dn *models.DeviceNetwork) {
	if !deviceJoinRequiresApproval(network, user) {
		return
	}
	dn.ApprovalRequired = true
	if dn.Status == models.DeviceNetworkStatusAvailable {
		dn.Status = models.DeviceNetworkStatusApprovalRequired
	}
}

func applyDeviceNetworkHostState(ctx context.Context, host *schema.Host, network schema.Network, user *schema.User, featureFlags models.FeatureFlags, dn *models.DeviceNetwork) {
	if pending, _ := getPendingHostOnNetwork(ctx, host.ID.String(), network.Name); pending != nil {
		dn.Pending = true
		dn.Status = models.DeviceNetworkStatusPending
		ts := pending.RequestedAt.Unix()
		dn.ApprovalRequestedAt = &ts
		return
	}

	if node, err := getHostNodeOnNetwork(ctx, host, network.Name); err == nil {
		dn.Joined = true
		dn.Connected = node.Connected
		if node.Connected {
			dn.Status = models.DeviceNetworkStatusJoined
		} else {
			dn.Status = models.DeviceNetworkStatusAvailable
		}
		return
	}

	violations, _ := CheckPostureViolationsForHost(host, nil, schema.NetworkID(network.Name), true)
	if len(violations) > 0 {
		dn.Status = models.DeviceNetworkStatusBlocked
		return
	}

	applyDeviceNetworkApprovalPolicy(network, user, featureFlags, dn)
}

func getPendingHostOnNetwork(ctx context.Context, hostID, network string) (*schema.PendingHost, error) {
	p := &schema.PendingHost{HostID: hostID, Network: network}
	if err := p.CheckIfPendingHostExists(ctx); err != nil {
		return nil, err
	}
	return p, nil
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
func JoinDeviceNetwork(ctx context.Context, user *schema.User, host *schema.Host, networkID string) (models.DeviceJoinResult, error) {
	var empty models.DeviceJoinResult
	if !UserHasAccessToNetwork(ctx, user, networkID) {
		return empty, errors.New("user does not have access to network")
	}
	hasAccess, _, err := CheckJITAccess(networkID, user.Username)
	if err != nil {
		return empty, err
	}
	if !hasAccess {
		return empty, errors.New("JIT access required: please request access from network admin")
	}

	network := &schema.Network{Name: networkID}
	if err := network.Get(ctx); err != nil {
		return empty, fmt.Errorf("network not found: %w", err)
	}
	if DoesHostExistInTheNetworkAlready(host, network) {
		return models.DeviceJoinResult{Status: models.DeviceJoinStatusJoined}, nil
	}

	violations, _ := CheckPostureViolationsForHost(host, nil, schema.NetworkID(networkID), true)
	if len(violations) > 0 {
		return empty, errors.New("access blocked: this device doesn't meet security requirements")
	}

	if deviceJoinRequiresApproval(*network, user) {
		p := &schema.PendingHost{HostID: host.ID.String(), Network: networkID}
		if err := p.CheckIfPendingHostExists(ctx); err == nil {
			return models.DeviceJoinResult{Status: models.DeviceJoinStatusPending}, nil
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
			return empty, err
		}
		return models.DeviceJoinResult{Status: models.DeviceJoinStatusPending}, nil
	}

	if pending, err := getPendingHostOnNetwork(ctx, host.ID.String(), networkID); err == nil && pending != nil {
		_ = pending.Delete(ctx)
	}

	JoinHostToNetworks(models.EnrollmentKey{
		Networks:           []string{networkID},
		SkipDeviceApproval: true,
	}, host, user.Username)
	return models.DeviceJoinResult{Status: models.DeviceJoinStatusJoined}, nil
}

// LeaveDeviceNetwork removes the host from a network or cancels a pending approval request.
func LeaveDeviceNetwork(ctx context.Context, user *schema.User, host *schema.Host, networkID string) error {
	if !UserHasAccessToNetwork(ctx, user, networkID) {
		return errors.New("user does not have access to network")
	}
	if pending, err := getPendingHostOnNetwork(ctx, host.ID.String(), networkID); err == nil && pending != nil {
		return pending.Delete(ctx)
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

// CancelDeviceNetworkJoin removes a pending join approval request without leaving a joined network.
func CancelDeviceNetworkJoin(ctx context.Context, user *schema.User, host *schema.Host, networkID string) error {
	if !UserHasAccessToNetwork(ctx, user, networkID) {
		return errors.New("user does not have access to network")
	}
	pending, err := getPendingHostOnNetwork(ctx, host.ID.String(), networkID)
	if err != nil {
		return errors.New("no pending join request for network")
	}
	return pending.Delete(ctx)
}

// SyncDevice requests the host to pull latest config via MQ.
func SyncDevice(host *schema.Host) error {
	if host == nil {
		return errors.New("host is required")
	}
	return RequestHostPullUpdate(host)
}

// RegisterDevice registers or updates a host on behalf of an authenticated user (Desktop/netclient JWT flow).
func RegisterDevice(ctx context.Context, user *schema.User, newHost *schema.Host) (models.RegisterResponse, error) {
	var empty models.RegisterResponse
	if user == nil || user.Username == "" {
		return empty, errors.New("user is required")
	}
	if newHost == nil || newHost.ID == uuid.Nil {
		return empty, errors.New("invalid host id")
	}
	if !IsVersionCompatible(newHost.Version) {
		return empty, fmt.Errorf("bad client version on register: %s", newHost.Version)
	}
	if newHost.TrafficKeyPublic == nil && newHost.OS != models.OS_Types.IoT {
		return empty, errors.New("missing traffic key")
	}

	trafficKey, err := RetrievePublicTrafficKey()
	if err != nil {
		return empty, err
	}

	var host *schema.Host
	if !HostExists(newHost) {
		newHost.PersistentKeepalive = models.DefaultPersistentKeepAlive
		newHost.OwnerUsername = user.Username
		_ = CheckHostPorts(newHost)
		if err := ProvisionDeviceHostMessaging(newHost); err != nil {
			return empty, err
		}
		if err := CreateHost(newHost); err != nil {
			return empty, err
		}
		host = newHost
	} else {
		existing := &schema.Host{ID: newHost.ID}
		if err := existing.Get(db.WithContext(ctx)); err != nil {
			return empty, err
		}
		if existing.OwnerUsername != "" && existing.OwnerUsername != user.Username {
			if err := TransferDeviceHostOwnership(ctx, existing, user.Username); err != nil {
				return empty, err
			}
			if err := existing.Get(db.WithContext(ctx)); err != nil {
				return empty, err
			}
		} else if existing.OwnerUsername == "" {
			EnsureHostOwner(existing, user.Username)
		}
		endpointChanged, _ := UpdateHostFromClient(newHost, existing)
		if endpointChanged {
			CheckHostPorts(existing)
		}
		if err := UpsertHost(existing); err != nil {
			return empty, err
		}
		host = existing
	}

	server := GetServerInfo()
	server.TrafficKey = trafficKey
	responseHost := *host
	responseHost.HostPass = ""
	return models.RegisterResponse{
		ServerConf:    server,
		RequestedHost: responseHost,
	}, nil
}
