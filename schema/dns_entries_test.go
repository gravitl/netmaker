package schema_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	testutils "github.com/gravitl/netmaker/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func createDNSEntry(t *testing.T, ctx context.Context, network *schema.Network, name string) *schema.DNSEntry {
	t.Helper()

	entry := &schema.DNSEntry{
		TenantID:  scope.ID(ctx),
		NetworkID: network.ID,
		Name:      name,
		Address:   "10.0.0.10",
	}
	require.NoError(t, entry.Create(ctx))
	return entry
}

func TestDNSEntry(t *testing.T) {
	ctx := setupSchemaTest(t)
	network := testutils.CreateIPv4Network(t, ctx, "net-1")
	other := testutils.CreateIPv4Network(t, ctx, "net-2")

	entry := createDNSEntry(t, ctx, network, "host-1")
	_, err := uuid.Parse(entry.ID)
	assert.NoError(t, err, "create generates a uuid id")
	createDNSEntry(t, ctx, network, "host-2")
	createDNSEntry(t, ctx, other, "host-1")

	t.Run("UniqueTenantNetworkName", func(t *testing.T) {
		duplicate := &schema.DNSEntry{TenantID: "tenant-1", NetworkID: network.ID, Name: "host-1"}
		assert.Error(t, duplicate.Create(ctx))
	})

	t.Run("Get", func(t *testing.T) {
		byNetworkName := &schema.DNSEntry{Network: &schema.Network{Name: "net-1"}, Name: "host-1"}
		require.NoError(t, byNetworkName.Get(ctx))
		assert.Equal(t, entry.ID, byNetworkName.ID)
		assert.Equal(t, "10.0.0.10", byNetworkName.Address)
		require.NotNil(t, byNetworkName.Network)
		assert.Equal(t, "net-1", byNetworkName.Network.Name)

		byNetworkID := &schema.DNSEntry{NetworkID: network.ID, Name: "host-1"}
		require.NoError(t, byNetworkID.Get(ctx))
		assert.Equal(t, entry.ID, byNetworkID.ID)

		assert.ErrorIs(t, (&schema.DNSEntry{ID: entry.ID}).Get(tenantCtx("tenant-2")), gorm.ErrRecordNotFound)
		assert.ErrorIs(t, (&schema.DNSEntry{Name: "host-1"}).Get(ctx), schema.ErrDNSEntryIdentifiersNotProvided)
	})

	t.Run("ListByNetwork", func(t *testing.T) {
		entries, err := (&schema.DNSEntry{Network: &schema.Network{Name: "net-1"}}).ListByNetwork(ctx)
		require.NoError(t, err)
		assert.Len(t, entries, 2)

		entries, err = (&schema.DNSEntry{NetworkID: other.ID}).ListByNetwork(ctx)
		require.NoError(t, err)
		assert.Len(t, entries, 1)
	})

	t.Run("Update", func(t *testing.T) {
		update := &schema.DNSEntry{ID: entry.ID, NetworkID: network.ID, Name: "host-1", Address6: "fd00::10"}
		require.NoError(t, update.Update(ctx))

		got := &schema.DNSEntry{ID: entry.ID}
		require.NoError(t, got.Get(ctx))
		assert.Empty(t, got.Address, "all fields are overwritten")
		assert.Equal(t, "fd00::10", got.Address6)
		assert.Equal(t, "tenant-1", got.TenantID)
	})

	t.Run("DeleteByNetwork", func(t *testing.T) {
		require.NoError(t, (&schema.DNSEntry{NetworkID: network.ID}).DeleteByNetwork(ctx))
		count, err := (&schema.DNSEntry{}).Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("Delete", func(t *testing.T) {
		require.NoError(t, (&schema.DNSEntry{NetworkID: other.ID, Name: "host-1"}).Delete(ctx))
		assert.ErrorIs(t, (&schema.DNSEntry{NetworkID: other.ID, Name: "host-1"}).Delete(ctx), gorm.ErrRecordNotFound)
	})
}
