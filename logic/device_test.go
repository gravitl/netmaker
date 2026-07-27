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
	ctx := db.WithContext(context.TODO())
	host := &schema.Host{ID: uuid.New(), Name: "ensure-owner-host", OwnerUsername: ""}
	require.NoError(t, host.Create(ctx))
	t.Cleanup(func() { _ = host.Delete(ctx) })

	EnsureHostOwner(host, "alice")
	assert.Equal(t, "alice", host.OwnerUsername)

	reloaded := &schema.Host{ID: host.ID}
	require.NoError(t, reloaded.Get(ctx))
	assert.Equal(t, "alice", reloaded.OwnerUsername)

	// Stale in-memory view must not overwrite an owner already set in DB.
	stale := &schema.Host{ID: host.ID, OwnerUsername: ""}
	EnsureHostOwner(stale, "carol")
	assert.Equal(t, "alice", stale.OwnerUsername)

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
	_, err = RegisterDevice(ctx, otherUser, dup)
	require.Error(t, err)
	assert.Equal(t, "host already registered to another user", err.Error())

	got, err := VerifyDeviceHostAccess(ctx, owner, hostID.String())
	require.NoError(t, err)
	assert.Equal(t, owner, got.OwnerUsername)

	_, err = VerifyDeviceHostAccess(ctx, other, hostID.String())
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

func TestTransferDeviceHostOwnershipCleansPending(t *testing.T) {
	ctx := db.WithContext(context.TODO())
	prevOwner := "handoff-prev-" + uuid.NewString()[:8]
	newOwner := "handoff-new-" + uuid.NewString()[:8]

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

	require.NoError(t, TransferDeviceHostOwnership(ctx, host, newOwner))
	assert.Equal(t, newOwner, host.OwnerUsername)

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

	assert.True(t, IsUserAllowedToJoinNetwork(ctx, username, "allowed-net"))
	assert.False(t, IsUserAllowedToJoinNetwork(ctx, username, "denied-net"))
	assert.False(t, IsUserAllowedToJoinNetwork(ctx, "", "allowed-net"))
}

func TestDeviceJoinRequiresApprovalWhenJITDisabled(t *testing.T) {
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

	user := &schema.User{Username: username, PlatformRoleID: schema.PlatformUser}
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

	origFilter := FilterNetworksByRole
	t.Cleanup(func() { FilterNetworksByRole = origFilter })
	FilterNetworksByRole = func(_ []schema.Network, _ *schema.User) []schema.Network {
		return []schema.Network{{Name: netName}}
	}

	origJITAccess := CheckJITAccess
	t.Cleanup(func() { CheckJITAccess = origJITAccess })
	CheckJITAccess = func(string, string) (bool, *schema.JITGrant, error) {
		return true, nil, nil
	}

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

	check := &schema.PendingHost{HostID: hostID.String(), Network: netName}
	require.NoError(t, check.CheckIfPendingHostExists(ctx))
}

func TestDeviceJoinSkipsApprovalWhenJITEnabledForUser(t *testing.T) {
	ctx := db.WithContext(context.TODO())
	netName := "jit-skip-approval-" + uuid.NewString()[:8]
	username := "jit-skip-user-" + uuid.NewString()[:8]

	origFlags := GetFeatureFlags
	t.Cleanup(func() { GetFeatureFlags = origFlags })
	GetFeatureFlags = func() models.FeatureFlags {
		flags := origFlags()
		flags.EnableDeviceApproval = true
		return flags
	}

	origJITSubject := UserSubjectToNetworkJIT
	t.Cleanup(func() { UserSubjectToNetworkJIT = origJITSubject })
	UserSubjectToNetworkJIT = func(networkID string, _ *schema.User) bool {
		return networkID == netName
	}

	origJITAccess := CheckJITAccess
	t.Cleanup(func() { CheckJITAccess = origJITAccess })
	CheckJITAccess = func(string, string) (bool, *schema.JITGrant, error) {
		return true, nil, nil
	}

	require.NoError(t, CreateNetwork(&schema.Network{
		Name:         netName,
		AddressRange: "10.98.0.0/24",
		AutoJoin:     false,
		JITEnabled:   true,
	}))
	t.Cleanup(func() { _ = (&schema.Network{Name: netName}).Delete(ctx) })

	user := &schema.User{Username: username, PlatformRoleID: schema.PlatformUser}
	require.NoError(t, user.Create(ctx))
	t.Cleanup(func() { _ = user.Delete(ctx) })

	hostID := uuid.New()
	host := &schema.Host{
		ID:               hostID,
		Name:             "jit-skip-host",
		OS:               "linux",
		Version:          "dev",
		HostPass:         "test-pass",
		TrafficKeyPublic: []byte{1, 2, 3},
		OwnerUsername:    username,
	}
	require.NoError(t, host.Create(ctx))
	t.Cleanup(func() { _ = host.Delete(ctx) })

	origFilter := FilterNetworksByRole
	t.Cleanup(func() { FilterNetworksByRole = origFilter })
	FilterNetworksByRole = func(_ []schema.Network, _ *schema.User) []schema.Network {
		return []schema.Network{{Name: netName, JITEnabled: true, AutoJoin: false}}
	}

	stalePending := schema.PendingHost{
		ID:          uuid.NewString(),
		HostID:      hostID.String(),
		Network:     netName,
		RequestedAt: time.Now().UTC(),
	}
	require.NoError(t, stalePending.Create(ctx))

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
	assert.False(t, found.ApprovalRequired)
	assert.False(t, found.Pending)
	assert.Equal(t, models.DeviceNetworkStatusAvailable, found.Status)

	var joinKey models.EnrollmentKey
	origJoin := JoinHostToNetworks
	JoinHostToNetworks = func(key models.EnrollmentKey, _ *schema.Host, _ string) {
		joinKey = key
	}
	t.Cleanup(func() { JoinHostToNetworks = origJoin })

	result, err := JoinDeviceNetwork(ctx, user, host, netName)
	require.NoError(t, err)
	assert.Equal(t, models.DeviceJoinStatusJoined, result.Status)
	assert.True(t, joinKey.SkipDeviceApproval)

	check := &schema.PendingHost{HostID: hostID.String(), Network: netName}
	require.Error(t, check.CheckIfPendingHostExists(ctx))
}

func TestDeviceJoinSkipsApprovalForNetworkAdmin(t *testing.T) {
	ctx := db.WithContext(context.TODO())
	netName := "admin-skip-approval-" + uuid.NewString()[:8]
	username := "admin-skip-user-" + uuid.NewString()[:8]

	origFlags := GetFeatureFlags
	t.Cleanup(func() { GetFeatureFlags = origFlags })
	GetFeatureFlags = func() models.FeatureFlags {
		flags := origFlags()
		flags.EnableDeviceApproval = true
		return flags
	}

	origIsAdmin := IsNetworkAdmin
	t.Cleanup(func() { IsNetworkAdmin = origIsAdmin })
	IsNetworkAdmin = func(_ *schema.User, networkID string) bool {
		return networkID == netName
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

	hostID := uuid.New()
	host := &schema.Host{
		ID:               hostID,
		Name:             "admin-skip-host",
		OS:               "linux",
		Version:          "dev",
		HostPass:         "test-pass",
		TrafficKeyPublic: []byte{1, 2, 3},
		OwnerUsername:    username,
	}
	require.NoError(t, host.Create(ctx))
	t.Cleanup(func() { _ = host.Delete(ctx) })

	origFilter := FilterNetworksByRole
	t.Cleanup(func() { FilterNetworksByRole = origFilter })
	FilterNetworksByRole = func(_ []schema.Network, _ *schema.User) []schema.Network {
		return []schema.Network{{Name: netName}}
	}

	origJITAccess := CheckJITAccess
	t.Cleanup(func() { CheckJITAccess = origJITAccess })
	CheckJITAccess = func(string, string) (bool, *schema.JITGrant, error) {
		return true, nil, nil
	}

	var joinKey models.EnrollmentKey
	origJoin := JoinHostToNetworks
	JoinHostToNetworks = func(key models.EnrollmentKey, _ *schema.Host, _ string) {
		joinKey = key
	}
	t.Cleanup(func() { JoinHostToNetworks = origJoin })

	result, err := JoinDeviceNetwork(ctx, user, host, netName)
	require.NoError(t, err)
	assert.Equal(t, models.DeviceJoinStatusJoined, result.Status)
	assert.True(t, joinKey.SkipDeviceApproval)
}

func TestCancelDeviceNetworkJoinClearsStalePending(t *testing.T) {
	ctx := db.WithContext(context.TODO())
	netName := "cancel-pending-" + uuid.NewString()[:8]
	username := "cancel-pending-user-" + uuid.NewString()[:8]

	require.NoError(t, CreateNetwork(&schema.Network{
		Name:         netName,
		AddressRange: "10.95.0.0/24",
	}))
	t.Cleanup(func() { _ = (&schema.Network{Name: netName}).Delete(ctx) })

	user := &schema.User{Username: username, PlatformRoleID: schema.AdminRole}
	require.NoError(t, user.Create(ctx))
	t.Cleanup(func() { _ = user.Delete(ctx) })

	hostID := uuid.New()
	host := &schema.Host{
		ID:               hostID,
		Name:             "cancel-pending-host",
		OS:               "linux",
		Version:          "dev",
		HostPass:         "test-pass",
		TrafficKeyPublic: []byte{1, 2, 3},
		OwnerUsername:    username,
	}
	require.NoError(t, host.Create(ctx))
	t.Cleanup(func() { _ = host.Delete(ctx) })

	pending := schema.PendingHost{
		ID:          uuid.NewString(),
		HostID:      hostID.String(),
		Network:     netName,
		RequestedAt: time.Now().UTC(),
	}
	require.NoError(t, pending.Create(ctx))

	require.NoError(t, CancelDeviceNetworkJoin(ctx, user, host, netName))

	check := &schema.PendingHost{HostID: hostID.String(), Network: netName}
	require.Error(t, check.CheckIfPendingHostExists(ctx))

	err := CancelDeviceNetworkJoin(ctx, user, host, netName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pending join request")
}

func TestJoinDeviceNetworkRequiresWriteAccess(t *testing.T) {
	ctx := db.WithContext(context.TODO())
	netName := "write-denied-" + uuid.NewString()[:8]
	username := "write-denied-user-" + uuid.NewString()[:8]

	require.NoError(t, CreateNetwork(&schema.Network{
		Name:         netName,
		AddressRange: "10.94.0.0/24",
	}))
	t.Cleanup(func() { _ = (&schema.Network{Name: netName}).Delete(ctx) })

	user := &schema.User{Username: username, PlatformRoleID: schema.PlatformUser}
	require.NoError(t, user.Create(ctx))
	t.Cleanup(func() { _ = user.Delete(ctx) })

	hostID := uuid.New()
	host := &schema.Host{
		ID:               hostID,
		Name:             "write-denied-host",
		OS:               "linux",
		Version:          "dev",
		HostPass:         "test-pass",
		TrafficKeyPublic: []byte{1, 2, 3},
		OwnerUsername:    username,
	}
	require.NoError(t, host.Create(ctx))
	t.Cleanup(func() { _ = (&schema.Host{ID: hostID}).Delete(ctx) })

	origFilter := FilterNetworksByRole
	origWrite := UserHasDeviceNetworkWriteAccess
	t.Cleanup(func() {
		FilterNetworksByRole = origFilter
		UserHasDeviceNetworkWriteAccess = origWrite
	})
	FilterNetworksByRole = func(_ []schema.Network, _ *schema.User) []schema.Network {
		return []schema.Network{{Name: netName}}
	}
	UserHasDeviceNetworkWriteAccess = func(_ context.Context, _ *schema.User, _ string) bool { return false }

	_, err := JoinDeviceNetwork(ctx, user, host, netName)
	require.Error(t, err)
	assert.Equal(t, "operation not permitted", err.Error())
}
