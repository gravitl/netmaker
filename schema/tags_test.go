package schema_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	testutils "github.com/gravitl/netmaker/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func createSchemaTag(t *testing.T, ctx context.Context, network *schema.Network, name string) *schema.Tag {
	t.Helper()

	tag := &schema.Tag{
		TenantID:  scope.ID(ctx),
		NetworkID: network.ID,
		Name:      name,
		ColorCode: "#ffffff",
	}
	require.NoError(t, tag.Create(ctx))
	return tag
}

func TestTag_Create(t *testing.T) {
	ctx := setupSchemaTest(t)
	network := testutils.CreateIPv4Network(t, ctx, "net-1")

	tag := createSchemaTag(t, ctx, network, "tag-1")
	_, err := uuid.Parse(tag.ID)
	assert.NoError(t, err, "create generates a uuid id")

	t.Run("UniqueTenantNetworkName", func(t *testing.T) {
		duplicate := &schema.Tag{TenantID: "tenant-1", NetworkID: network.ID, Name: "tag-1"}
		assert.Error(t, duplicate.Create(ctx))
	})

	t.Run("SameNameInAnotherNetwork", func(t *testing.T) {
		createSchemaTag(t, ctx, testutils.CreateIPv4Network(t, ctx, "net-2"), "tag-1")
	})

	t.Run("SameNameInAnotherTenant", func(t *testing.T) {
		otherCtx := tenantCtx("tenant-2")
		createSchemaTag(t, otherCtx, testutils.CreateIPv4Network(t, otherCtx, "net-1"), "tag-1")
	})
}

func TestTag_Get(t *testing.T) {
	ctx := setupSchemaTest(t)
	network := testutils.CreateIPv4Network(t, ctx, "net-1")
	tag := createSchemaTag(t, ctx, network, "tag-1")

	byID := &schema.Tag{ID: tag.ID}
	require.NoError(t, byID.Get(ctx))
	assert.Equal(t, "tag-1", byID.Name)
	assert.Equal(t, "#ffffff", byID.ColorCode)

	byNetworkID := &schema.Tag{NetworkID: network.ID, Name: "tag-1"}
	require.NoError(t, byNetworkID.Get(ctx))
	assert.Equal(t, tag.ID, byNetworkID.ID)

	byNetworkName := &schema.Tag{Network: &schema.Network{Name: "net-1"}, Name: "tag-1"}
	require.NoError(t, byNetworkName.Get(ctx))
	assert.Equal(t, tag.ID, byNetworkName.ID)
	require.NotNil(t, byNetworkName.Network)
	assert.Equal(t, network.ID, byNetworkName.Network.ID)

	assert.ErrorIs(t, (&schema.Tag{ID: tag.ID}).Get(tenantCtx("tenant-2")), gorm.ErrRecordNotFound)
	assert.ErrorIs(t, (&schema.Tag{NetworkID: network.ID, Name: "missing"}).Get(ctx), gorm.ErrRecordNotFound)

	for _, tag := range []*schema.Tag{
		{},
		{Name: "tag-1"},
		{NetworkID: network.ID},
		{Network: &schema.Network{Name: "net-1"}},
	} {
		assert.ErrorIs(t, tag.Get(ctx), schema.ErrTagIdentifiersNotProvided)
	}
	assert.ErrorIs(t,
		(&schema.Tag{Network: &schema.Network{Name: "net-1"}, Name: "tag-1"}).Get(db.WithContext(context.Background())),
		schema.ErrTagIdentifiersNotProvided,
		"network names need a tenant",
	)
}

func TestTag_UpdateAndDelete(t *testing.T) {
	ctx := setupSchemaTest(t)
	network := testutils.CreateIPv4Network(t, ctx, "net-1")
	tag := createSchemaTag(t, ctx, network, "tag-1")

	update := &schema.Tag{ID: tag.ID, NetworkID: network.ID, Name: "tag-renamed", ColorCode: "#000000"}
	require.NoError(t, update.Update(ctx))

	got := &schema.Tag{ID: tag.ID}
	require.NoError(t, got.Get(ctx))
	assert.Equal(t, "tag-renamed", got.Name)
	assert.Equal(t, "#000000", got.ColorCode)
	assert.Equal(t, "tenant-1", got.TenantID, "the tenant is not overwritten")

	assert.ErrorIs(t, (&schema.Tag{ID: uuid.NewString(), NetworkID: network.ID, Name: "x"}).Update(ctx), gorm.ErrRecordNotFound)

	assert.ErrorIs(t, (&schema.Tag{ID: tag.ID}).Delete(tenantCtx("tenant-2")), gorm.ErrRecordNotFound)
	require.NoError(t, (&schema.Tag{NetworkID: network.ID, Name: "tag-renamed"}).Delete(ctx))
	assert.ErrorIs(t, (&schema.Tag{ID: tag.ID}).Get(ctx), gorm.ErrRecordNotFound)
}

func TestTag_ListAndDeleteByNetwork(t *testing.T) {
	ctx := setupSchemaTest(t)
	network := testutils.CreateIPv4Network(t, ctx, "net-1")
	other := testutils.CreateIPv4Network(t, ctx, "net-2")
	createSchemaTag(t, ctx, network, "tag-1")
	createSchemaTag(t, ctx, network, "tag-2")
	createSchemaTag(t, ctx, other, "tag-1")

	byName, err := (&schema.Tag{Network: &schema.Network{Name: "net-1"}}).ListByNetwork(ctx)
	require.NoError(t, err)
	require.Len(t, byName, 2)
	for _, tag := range byName {
		require.NotNil(t, tag.Network)
		assert.Equal(t, "net-1", tag.Network.Name)
	}

	byID, err := (&schema.Tag{NetworkID: other.ID}).ListByNetwork(ctx)
	require.NoError(t, err)
	assert.Len(t, byID, 1)

	require.NoError(t, (&schema.Tag{NetworkID: network.ID}).DeleteByNetwork(ctx))
	count, err := (&schema.Tag{}).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
