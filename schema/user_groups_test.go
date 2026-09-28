package schema_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUserGroup_Create(t *testing.T) {
	ctx := setupSchemaTest(t)

	custom := &schema.UserGroup{TenantID: "tenant-1", Name: "custom"}
	require.NoError(t, custom.Create(ctx))
	_, err := uuid.Parse(custom.ID)
	assert.NoError(t, err, "create generates a uuid id")
	assert.Equal(t, schema.UserGroupID(custom.ID), custom.Slug, "the slug defaults to the id")

	defaultGroup := &schema.UserGroup{TenantID: "tenant-1", Slug: "net-1-network-admin-grp", Name: "default"}
	require.NoError(t, defaultGroup.Create(ctx))
	assert.NotEqual(t, defaultGroup.Slug, schema.UserGroupID(defaultGroup.ID))

	t.Run("UniqueTenantSlug", func(t *testing.T) {
		duplicate := &schema.UserGroup{TenantID: "tenant-1", Slug: "net-1-network-admin-grp"}
		assert.Error(t, duplicate.Create(ctx))

		otherTenant := &schema.UserGroup{TenantID: "tenant-2", Slug: "net-1-network-admin-grp"}
		assert.NoError(t, otherTenant.Create(tenantCtx("tenant-2")))
	})
}

func TestUserGroup_Get(t *testing.T) {
	ctx := setupSchemaTest(t)
	group := &schema.UserGroup{TenantID: "tenant-1", Slug: "net-1-network-admin-grp", Name: "admins"}
	require.NoError(t, group.Create(ctx))

	bySlug := &schema.UserGroup{Slug: "net-1-network-admin-grp"}
	require.NoError(t, bySlug.Get(ctx))
	assert.Equal(t, group.ID, bySlug.ID)
	assert.Equal(t, "admins", bySlug.Name)

	byID := &schema.UserGroup{ID: group.ID}
	require.NoError(t, byID.Get(ctx))
	assert.Equal(t, schema.UserGroupID("net-1-network-admin-grp"), byID.Slug)

	byName := &schema.UserGroup{Name: "admins"}
	require.NoError(t, byName.GetByName(ctx))
	assert.Equal(t, group.ID, byName.ID)
	assert.Equal(t, schema.UserGroupID("net-1-network-admin-grp"), byName.Slug, "the slug is returned without a tenant prefix")

	assert.ErrorIs(t, (&schema.UserGroup{Slug: "net-1-network-admin-grp"}).Get(tenantCtx("tenant-2")), gorm.ErrRecordNotFound)
	assert.ErrorIs(t, (&schema.UserGroup{ID: group.ID}).Get(tenantCtx("tenant-2")), gorm.ErrRecordNotFound)
	assert.ErrorIs(t, (&schema.UserGroup{}).Get(ctx), schema.ErrUserGroupIdentifiersNotProvided)
}

func TestUserGroup_UpsertAndUpdate(t *testing.T) {
	ctx := setupSchemaTest(t)

	group := &schema.UserGroup{TenantID: "tenant-1", Slug: "global-network-admin-grp", Name: "v1", Default: true}
	require.NoError(t, group.Upsert(ctx))
	id := group.ID
	_, err := uuid.Parse(id)
	require.NoError(t, err, "upsert creates the group with a uuid id")

	again := &schema.UserGroup{TenantID: "tenant-1", Slug: "global-network-admin-grp", Name: "v2", Default: true}
	require.NoError(t, again.Upsert(ctx))
	assert.Equal(t, id, again.ID, "upserting by slug keeps the id")

	count, err := (&schema.UserGroup{}).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	update := &schema.UserGroup{Slug: "global-network-admin-grp", MetaData: "updated"}
	require.NoError(t, update.Update(ctx))

	got := &schema.UserGroup{Slug: "global-network-admin-grp"}
	require.NoError(t, got.Get(ctx))
	assert.Equal(t, "v2", got.Name)
	assert.Equal(t, "updated", got.MetaData, "update by slug")
	assert.Equal(t, id, got.ID)
}

func TestUserGroup_DeleteAndList(t *testing.T) {
	ctx := setupSchemaTest(t)
	otherCtx := tenantCtx("tenant-2")
	require.NoError(t, (&schema.UserGroup{TenantID: "tenant-1", Slug: "grp-1"}).Create(ctx))
	require.NoError(t, (&schema.UserGroup{TenantID: "tenant-1", Slug: "grp-2"}).Create(ctx))
	require.NoError(t, (&schema.UserGroup{TenantID: "tenant-2", Slug: "grp-1"}).Create(otherCtx))

	groups, err := (&schema.UserGroup{}).ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	for _, group := range groups {
		assert.Equal(t, "tenant-1", group.TenantID)
	}

	require.NoError(t, (&schema.UserGroup{Slug: "grp-1"}).Delete(ctx))
	assert.ErrorIs(t, (&schema.UserGroup{Slug: "grp-1"}).Get(ctx), gorm.ErrRecordNotFound)
	assert.NoError(t, (&schema.UserGroup{Slug: "grp-1"}).Get(otherCtx), "delete is tenant scoped")

	count, err := (&schema.UserGroup{}).Count(db.WithContext(ctx))
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
