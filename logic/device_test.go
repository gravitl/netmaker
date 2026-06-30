package logic

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
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
