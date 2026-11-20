package logic

import (
	"context"
	"testing"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTagTest(t *testing.T) context.Context {
	t.Helper()
	t.Chdir(t.TempDir())
	require.NoError(t, db.InitializeDB(schema.ListModels()...))
	t.Cleanup(db.CloseDB)

	return scope.WithContext(db.WithContext(context.Background()), scope.TenantScope, "tenant-1")
}

func createTagTestNetwork(t *testing.T, ctx context.Context, name string) *schema.Network {
	t.Helper()

	network := &schema.Network{TenantID: scope.ID(ctx), Name: name, AddressRange: "10.0.0.0/24"}
	require.NoError(t, network.Create(ctx))
	return network
}

func TestTags(t *testing.T) {
	ctx := setupTagTest(t)
	createTagTestNetwork(t, ctx, "net-1")
	createTagTestNetwork(t, ctx, "net-2")

	tag := models.Tag{
		ID:        "net-1.tag-1",
		TagName:   "tag-1",
		Network:   "net-1",
		ColorCode: "#ffffff",
		CreatedBy: "admin",
	}
	require.NoError(t, InsertTag(ctx, tag))
	assert.Error(t, InsertTag(ctx, tag), "inserting an existing tag fails")

	got, err := GetTag(ctx, "net-1.tag-1")
	require.NoError(t, err)
	assert.Equal(t, models.TagID("net-1.tag-1"), got.ID)
	assert.Equal(t, "tag-1", got.TagName)
	assert.Equal(t, schema.NetworkID("net-1"), got.Network)
	assert.Equal(t, "#ffffff", got.ColorCode)
	assert.Equal(t, "admin", got.CreatedBy)

	t.Run("NotFound", func(t *testing.T) {
		for _, tagID := range []models.TagID{"net-2.tag-1", "net-1.missing", "no-network", "missing-net.tag-1"} {
			_, err := GetTag(ctx, tagID)
			assert.ErrorIs(t, err, gorm.ErrRecordNotFound, tagID)
		}
	})

	t.Run("Upsert", func(t *testing.T) {
		tag.ColorCode = "#000000"
		require.NoError(t, UpsertTag(ctx, tag))
		got, err := GetTag(ctx, "net-1.tag-1")
		require.NoError(t, err)
		assert.Equal(t, "#000000", got.ColorCode)

		require.NoError(t, UpsertTag(ctx, newTag("net-2", "tag-2")))
		_, err = GetTag(ctx, "net-2.tag-2")
		assert.NoError(t, err, "upsert creates missing tags")
	})

	t.Run("ListNetworkTags", func(t *testing.T) {
		require.NoError(t, InsertTag(ctx, newTag("net-1", "tag-3")))

		tags, err := ListNetworkTags(ctx, "net-1")
		require.NoError(t, err)
		ids := make([]models.TagID, 0, len(tags))
		for _, tag := range tags {
			ids = append(ids, tag.ID)
		}
		assert.ElementsMatch(t, []models.TagID{"net-1.tag-1", "net-1.tag-3"}, ids)
	})

	t.Run("Delete", func(t *testing.T) {
		require.NoError(t, DeleteTag(ctx, "net-1.tag-3", false))
		_, err := GetTag(ctx, "net-1.tag-3")
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("OtherTenant", func(t *testing.T) {
		otherCtx := scope.WithContext(ctx, scope.TenantScope, "tenant-2")
		createTagTestNetwork(t, otherCtx, "net-1")

		_, err := GetTag(otherCtx, "net-1.tag-1")
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
		require.NoError(t, InsertTag(otherCtx, tag), "the same tag id in another tenant")
	})
}

func newTag(network, name string) models.Tag {
	return models.Tag{
		ID:      models.TagID(network + "." + name),
		TagName: name,
		Network: schema.NetworkID(network),
	}
}
