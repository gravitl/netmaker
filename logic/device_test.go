package logic

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureHostOwner(t *testing.T) {
	host := &schema.Host{ID: uuid.New(), OwnerUsername: ""}
	EnsureHostOwner(host, "alice")
	assert.Equal(t, "alice", host.OwnerUsername)

	host.OwnerUsername = "bob"
	EnsureHostOwner(host, "alice")
	assert.Equal(t, "bob", host.OwnerUsername)

	EnsureHostOwner(nil, "alice")
	EnsureHostOwner(host, "")
}

func TestVerifyDeviceHostAccess(t *testing.T) {
	ctx := db.WithContext(context.TODO())
	owner := "device-user-" + uuid.NewString()[:8]
	other := "other-user-" + uuid.NewString()[:8]

	host := &schema.Host{
		ID:            uuid.New(),
		Name:          "test-host",
		OwnerUsername: owner,
	}
	require.NoError(t, host.Create(ctx))
	t.Cleanup(func() { _ = host.Delete(ctx) })

	t.Run("owner allowed", func(t *testing.T) {
		got, err := VerifyDeviceHostAccess(ctx, owner, host.ID.String())
		require.NoError(t, err)
		assert.Equal(t, host.ID, got.ID)
	})

	t.Run("other user denied", func(t *testing.T) {
		_, err := VerifyDeviceHostAccess(ctx, other, host.ID.String())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not belong")
	})

	t.Run("claim unowned host", func(t *testing.T) {
		unowned := &schema.Host{ID: uuid.New(), Name: "unowned-host"}
		require.NoError(t, unowned.Create(ctx))
		t.Cleanup(func() { _ = unowned.Delete(ctx) })

		got, err := VerifyDeviceHostAccess(ctx, owner, unowned.ID.String())
		require.NoError(t, err)
		assert.Equal(t, owner, got.OwnerUsername)
	})

	t.Run("invalid host id", func(t *testing.T) {
		_, err := VerifyDeviceHostAccess(ctx, owner, "not-a-uuid")
		require.Error(t, err)
	})
}

func TestRegisterDevice(t *testing.T) {
	ctx := db.WithContext(context.TODO())
	owner := "reg-user-" + uuid.NewString()[:8]
	other := "other-reg-" + uuid.NewString()[:8]

	user := &schema.User{
		Username:       owner,
		PlatformRoleID: schema.AdminRole,
	}
	require.NoError(t, user.Create(ctx))
	t.Cleanup(func() { _ = user.Delete(ctx) })

	hostID := uuid.New()
	host := &schema.Host{
		ID:               hostID,
		Name:             "device-reg-host",
		OS:               "linux",
		Version:          "dev",
		HostPass:         "test-host-pass",
		TrafficKeyPublic: []byte{1, 2, 3},
	}
	resp, err := RegisterDevice(ctx, user, host)
	require.NoError(t, err)
	assert.Equal(t, hostID, resp.RequestedHost.ID)
	assert.Equal(t, owner, resp.RequestedHost.OwnerUsername)

	otherUser := &schema.User{Username: other, PlatformRoleID: schema.AdminRole}
	require.NoError(t, otherUser.Create(ctx))
	t.Cleanup(func() { _ = otherUser.Delete(ctx) })

	dup := &schema.Host{ID: hostID, Name: "device-reg-host", OS: "linux", Version: "dev", TrafficKeyPublic: []byte{1, 2, 3}}
	handoffResp, err := RegisterDevice(ctx, otherUser, dup)
	require.NoError(t, err)
	assert.Equal(t, other, handoffResp.RequestedHost.OwnerUsername)

	got, err := VerifyDeviceHostAccess(ctx, other, hostID.String())
	require.NoError(t, err)
	assert.Equal(t, other, got.OwnerUsername)

	_, err = VerifyDeviceHostAccess(ctx, owner, hostID.String())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong")
	t.Cleanup(func() { _ = (&schema.Host{ID: hostID}).Delete(ctx) })
}

func TestTransferDeviceHostOwnership_logsAuditEvent(t *testing.T) {
	ctx := db.WithContext(context.TODO())
	prevOwner := "audit-prev-" + uuid.NewString()[:8]
	newOwner := "audit-new-" + uuid.NewString()[:8]

	hostID := uuid.New()
	host := &schema.Host{
		ID:               hostID,
		Name:             "audit-handoff-host",
		OS:               "linux",
		Version:          "dev",
		HostPass:         "test-pass",
		TrafficKeyPublic: []byte{1, 2, 3},
		OwnerUsername:    prevOwner,
	}
	require.NoError(t, host.Create(ctx))
	t.Cleanup(func() { _ = (&schema.Host{ID: hostID}).Delete(ctx) })

	var logged models.Event
	origLogEvent := LogEvent
	LogEvent = func(e *models.Event) {
		logged = *e
	}
	t.Cleanup(func() { LogEvent = origLogEvent })

	require.NoError(t, TransferDeviceHostOwnership(ctx, host, newOwner))
	assert.Equal(t, schema.TransferDeviceOwnership, logged.Action)
	assert.Equal(t, newOwner, logged.TriggeredBy)
	assert.Equal(t, hostID.String(), logged.Target.ID)
	assert.Equal(t, schema.DeviceSub, logged.Target.Type)
	assert.Equal(t, schema.ClientApp, logged.Origin)
	oldDiff, ok := logged.Diff.Old.(map[string]string)
	require.True(t, ok)
	assert.Equal(t, prevOwner, oldDiff["owner_username"])
	newDiff, ok := logged.Diff.New.(map[string]string)
	require.True(t, ok)
	assert.Equal(t, newOwner, newDiff["owner_username"])
}

func TestRegisterDeviceHostHandoffCleansPending(t *testing.T) {
	ctx := db.WithContext(context.TODO())
	prevOwner := "handoff-prev-" + uuid.NewString()[:8]
	newOwner := "handoff-new-" + uuid.NewString()[:8]

	prevUser := &schema.User{Username: prevOwner, PlatformRoleID: schema.AdminRole}
	require.NoError(t, prevUser.Create(ctx))
	t.Cleanup(func() { _ = prevUser.Delete(ctx) })

	newUser := &schema.User{Username: newOwner, PlatformRoleID: schema.AdminRole}
	require.NoError(t, newUser.Create(ctx))
	t.Cleanup(func() { _ = newUser.Delete(ctx) })

	hostID := uuid.New()
	host := &schema.Host{
		ID:               hostID,
		Name:             "handoff-host",
		OS:               "linux",
		Version:          "dev",
		HostPass:         "test-pass",
		TrafficKeyPublic: []byte{1, 2, 3},
		OwnerUsername:    prevOwner,
	}
	require.NoError(t, host.Create(ctx))
	t.Cleanup(func() { _ = (&schema.Host{ID: hostID}).Delete(ctx) })

	netName := "handoff-net-" + uuid.NewString()[:8]
	require.NoError(t, CreateNetwork(&schema.Network{
		Name:         netName,
		AddressRange: "10.98.0.0/24",
	}))
	t.Cleanup(func() { _ = (&schema.Network{Name: netName}).Delete(ctx) })

	pending := schema.PendingHost{
		ID:      uuid.NewString(),
		HostID:  hostID.String(),
		Network: netName,
	}
	require.NoError(t, pending.Create(ctx))

	dup := &schema.Host{ID: hostID, Name: "handoff-host", OS: "linux", Version: "dev", TrafficKeyPublic: []byte{1, 2, 3}}
	resp, err := RegisterDevice(ctx, newUser, dup)
	require.NoError(t, err)
	assert.Equal(t, newOwner, resp.RequestedHost.OwnerUsername)

	check := &schema.PendingHost{HostID: hostID.String(), Network: netName}
	require.Error(t, check.CheckIfPendingHostExists(ctx))
}

func TestIsUserAllowedToJoinNetworkUsesRoleFilter(t *testing.T) {
	ctx := db.WithContext(context.TODO())
	username := "join-test-" + uuid.NewString()[:8]
	user := &schema.User{
		Username:       username,
		PlatformRoleID: schema.AdminRole,
	}
	require.NoError(t, user.Create(ctx))
	t.Cleanup(func() { _ = user.Delete(ctx) })

	origFilter := FilterNetworksByRole
	t.Cleanup(func() { FilterNetworksByRole = origFilter })

	FilterNetworksByRole = func(_ []schema.Network, _ *schema.User) []schema.Network {
		return []schema.Network{{Name: "allowed-net"}}
	}

	assert.True(t, IsUserAllowedToJoinNetwork(username, "allowed-net"))
	assert.False(t, IsUserAllowedToJoinNetwork(username, "denied-net"))
	assert.False(t, IsUserAllowedToJoinNetwork("", "allowed-net"))
}

func TestGetDeviceNetworksApprovalRequiredWithoutHost(t *testing.T) {
	ctx := db.WithContext(context.TODO())
	netName := "approval-nohost-" + uuid.NewString()[:8]
	username := "approval-nohost-user-" + uuid.NewString()[:8]

	origFlags := GetFeatureFlags
	t.Cleanup(func() { GetFeatureFlags = origFlags })
	GetFeatureFlags = func() models.FeatureFlags {
		flags := origFlags()
		flags.EnableDeviceApproval = true
		return flags
	}

	require.NoError(t, CreateNetwork(&schema.Network{
		Name:         netName,
		AddressRange: "10.97.0.0/24",
		AutoJoin:     false,
	}))
	t.Cleanup(func() { _ = (&schema.Network{Name: netName}).Delete(ctx) })

	user := &schema.User{Username: username, PlatformRoleID: schema.AdminRole}
	require.NoError(t, user.Create(ctx))
	t.Cleanup(func() { _ = user.Delete(ctx) })

	networks, err := GetDeviceNetworks(ctx, user, nil)
	require.NoError(t, err)
	var found models.DeviceNetwork
	for _, n := range networks {
		if n.NetworkID == netName {
			found = n
			break
		}
	}
	require.Equal(t, netName, found.NetworkID)
	assert.True(t, found.ApprovalRequired)
	assert.Equal(t, models.DeviceNetworkStatusApprovalRequired, found.Status)
}

func TestGetDeviceNetworksShowsPendingApproval(t *testing.T) {
	ctx := db.WithContext(context.TODO())
	netName := "pending-list-" + uuid.NewString()[:8]
	username := "pending-list-user-" + uuid.NewString()[:8]

	origFlags := GetFeatureFlags
	t.Cleanup(func() { GetFeatureFlags = origFlags })
	GetFeatureFlags = func() models.FeatureFlags {
		flags := origFlags()
		flags.EnableDeviceApproval = true
		return flags
	}

	require.NoError(t, CreateNetwork(&schema.Network{
		Name:         netName,
		AddressRange: "10.96.0.0/24",
		AutoJoin:     false,
	}))
	t.Cleanup(func() { _ = (&schema.Network{Name: netName}).Delete(ctx) })

	user := &schema.User{Username: username, PlatformRoleID: schema.AdminRole}
	require.NoError(t, user.Create(ctx))
	t.Cleanup(func() { _ = user.Delete(ctx) })

	hostID := uuid.New()
	host := &schema.Host{
		ID:               hostID,
		Name:             "pending-list-host",
		OS:               "linux",
		Version:          "dev",
		HostPass:         "test-pass",
		TrafficKeyPublic: []byte{1, 2, 3},
		OwnerUsername:    username,
	}
	require.NoError(t, host.Create(ctx))
	t.Cleanup(func() { _ = (&schema.Host{ID: hostID}).Delete(ctx) })

	pending := schema.PendingHost{
		ID:          uuid.NewString(),
		HostID:      hostID.String(),
		Network:     netName,
		RequestedAt: time.Now().UTC(),
	}
	require.NoError(t, pending.Create(ctx))
	t.Cleanup(func() { _ = pending.Delete(ctx) })

	networks, err := GetDeviceNetworks(ctx, user, host)
	require.NoError(t, err)
	var found models.DeviceNetwork
	for _, n := range networks {
		if n.NetworkID == netName {
			found = n
			break
		}
	}
	require.Equal(t, netName, found.NetworkID)
	assert.True(t, found.Pending)
	assert.Equal(t, models.DeviceNetworkStatusPending, found.Status)
	assert.NotNil(t, found.ApprovalRequestedAt)
}

func TestDeviceApprovalFlow(t *testing.T) {
	ctx := db.WithContext(context.TODO())
	netName := "approval-net-" + uuid.NewString()[:8]
	username := "approval-user-" + uuid.NewString()[:8]

	origFlags := GetFeatureFlags
	t.Cleanup(func() { GetFeatureFlags = origFlags })
	GetFeatureFlags = func() models.FeatureFlags {
		flags := origFlags()
		flags.EnableDeviceApproval = true
		return flags
	}

	require.NoError(t, CreateNetwork(&schema.Network{
		Name:         netName,
		AddressRange: "10.99.0.0/24",
		AutoJoin:     false,
	}))
	t.Cleanup(func() { _ = (&schema.Network{Name: netName}).Delete(ctx) })

	user := &schema.User{Username: username, PlatformRoleID: schema.AdminRole}
	require.NoError(t, user.Create(ctx))
	t.Cleanup(func() { _ = user.Delete(ctx) })

	hostID := uuid.New()
	host := &schema.Host{
		ID:               hostID,
		Name:             "approval-host",
		OS:               "linux",
		Version:          "dev",
		HostPass:         "test-pass",
		TrafficKeyPublic: []byte{1, 2, 3},
		OwnerUsername:    username,
	}
	require.NoError(t, host.Create(ctx))
	t.Cleanup(func() { _ = host.Delete(ctx) })

	networks, err := GetDeviceNetworks(ctx, user, host)
	require.NoError(t, err)
	var found models.DeviceNetwork
	for _, n := range networks {
		if n.NetworkID == netName {
			found = n
			break
		}
	}
	require.Equal(t, netName, found.NetworkID)
	assert.True(t, found.ApprovalRequired)
	assert.Equal(t, models.DeviceNetworkStatusApprovalRequired, found.Status)

	result, err := JoinDeviceNetwork(ctx, user, host, netName)
	require.NoError(t, err)
	assert.Equal(t, models.DeviceJoinStatusPending, result.Status)

	result, err = JoinDeviceNetwork(ctx, user, host, netName)
	require.NoError(t, err)
	assert.Equal(t, models.DeviceJoinStatusPending, result.Status)

	networks, err = GetDeviceNetworks(ctx, user, host)
	require.NoError(t, err)
	for _, n := range networks {
		if n.NetworkID == netName {
			assert.True(t, n.Pending)
			assert.Equal(t, models.DeviceNetworkStatusPending, n.Status)
			assert.NotNil(t, n.ApprovalRequestedAt)
			break
		}
	}

	require.NoError(t, LeaveDeviceNetwork(ctx, user, host, netName))

	networks, err = GetDeviceNetworks(ctx, user, host)
	require.NoError(t, err)
	for _, n := range networks {
		if n.NetworkID == netName {
			assert.False(t, n.Pending)
			assert.True(t, n.ApprovalRequired)
			assert.Equal(t, models.DeviceNetworkStatusApprovalRequired, n.Status)
			break
		}
	}

	_, err = JoinDeviceNetwork(ctx, user, host, netName)
	require.NoError(t, err)

	err = CancelDeviceNetworkJoin(ctx, user, host, netName)
	require.NoError(t, err)

	err = CancelDeviceNetworkJoin(ctx, user, host, netName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pending join request")
}
