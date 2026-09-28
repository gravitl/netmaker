package schema_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUserRole_PlatformRoles(t *testing.T) {
	ctx := setupSchemaTest(t)

	superAdmin := &schema.UserRole{Slug: schema.SuperAdminRole, Default: true, TenantGlobalAccess: true}
	require.NoError(t, superAdmin.Upsert(ctx))
	_, err := uuid.Parse(superAdmin.ID)
	require.NoError(t, err, "upsert creates the role with a uuid id")
	assert.Equal(t, scope.GlobalScope, superAdmin.Scope, "platform roles are global")
	assert.Empty(t, superAdmin.ScopeID, "platform roles are shared by all the tenants")

	orgOwner := &schema.UserRole{Slug: schema.OrgOwner, Default: true}
	require.NoError(t, orgOwner.Upsert(ctx))
	assert.Equal(t, scope.GlobalScope, orgOwner.Scope, "org roles are global")
	assert.Empty(t, orgOwner.ScopeID)

	for _, tenantCtx := range []context.Context{ctx, tenantCtx("tenant-2"), db.WithContext(context.Background())} {
		got := &schema.UserRole{Slug: schema.SuperAdminRole}
		require.NoError(t, got.GetPlatformRole(tenantCtx))
		assert.Equal(t, superAdmin.ID, got.ID)
		assert.True(t, got.TenantGlobalAccess)
	}

	again := &schema.UserRole{Slug: schema.SuperAdminRole, Default: true, MetaData: "updated"}
	require.NoError(t, again.Upsert(tenantCtx("tenant-2")))
	assert.Equal(t, superAdmin.ID, again.ID, "upserting by slug keeps the id")

	roles, err := (&schema.UserRole{}).ListPlatformRoles(ctx)
	require.NoError(t, err)
	assert.Len(t, roles, 2)

	assert.ErrorIs(t, (&schema.UserRole{Slug: "missing"}).GetPlatformRole(ctx), gorm.ErrRecordNotFound)
	assert.ErrorIs(t, (&schema.UserRole{}).GetPlatformRole(ctx), schema.ErrUserRoleIdentifiersNotProvided)
}

func TestUserRole_NetworkRoles(t *testing.T) {
	ctx := setupSchemaTest(t)
	otherCtx := tenantCtx("tenant-2")

	admin := &schema.UserRole{Slug: "net-1-network-admin", NetworkID: "net-1"}
	require.NoError(t, admin.Upsert(ctx))
	assert.Equal(t, scope.TenantScope, admin.Scope)
	assert.Equal(t, "tenant-1", admin.ScopeID, "network roles are scoped to the tenant")
	require.NoError(t, (&schema.UserRole{Slug: "net-1-network-user", NetworkID: "net-1"}).Upsert(ctx))
	require.NoError(t, (&schema.UserRole{Slug: "net-2-network-admin", NetworkID: "net-2"}).Upsert(ctx))

	// the same slug in another tenant.
	otherAdmin := &schema.UserRole{Slug: "net-1-network-admin", NetworkID: "net-1"}
	require.NoError(t, otherAdmin.Upsert(otherCtx))
	assert.NotEqual(t, admin.ID, otherAdmin.ID)

	t.Run("Get", func(t *testing.T) {
		got := &schema.UserRole{Slug: "net-1-network-admin"}
		require.NoError(t, got.GetNetworkRole(ctx))
		assert.Equal(t, admin.ID, got.ID)

		byID := &schema.UserRole{ID: admin.ID}
		require.NoError(t, byID.GetNetworkRole(ctx))
		assert.Equal(t, schema.UserRoleID("net-1-network-admin"), byID.Slug)

		assert.ErrorIs(t, (&schema.UserRole{ID: admin.ID}).GetNetworkRole(otherCtx), gorm.ErrRecordNotFound)
		assert.ErrorIs(t, (&schema.UserRole{Slug: "net-1-network-admin"}).GetPlatformRole(ctx), gorm.ErrRecordNotFound,
			"network roles are not platform roles")
	})

	t.Run("UniqueScopeSlug", func(t *testing.T) {
		duplicate := &schema.UserRole{
			ID:        uuid.NewString(),
			Scope:     scope.TenantScope,
			ScopeID:   "tenant-1",
			Slug:      "net-1-network-admin",
			NetworkID: "net-1",
		}
		assert.Error(t, db.FromContext(ctx).Create(duplicate).Error)
	})

	t.Run("List", func(t *testing.T) {
		roles, err := (&schema.UserRole{}).ListNetworkRoles(ctx)
		require.NoError(t, err)
		assert.Len(t, roles, 3)

		roles, err = (&schema.UserRole{}).ListNetworkRoles(otherCtx)
		require.NoError(t, err)
		assert.Len(t, roles, 1)
	})

	t.Run("Delete", func(t *testing.T) {
		require.NoError(t, (&schema.UserRole{NetworkID: "net-1"}).DeleteNetworkRoles(ctx))
		roles, err := (&schema.UserRole{}).ListNetworkRoles(ctx)
		require.NoError(t, err)
		require.Len(t, roles, 1)
		assert.Equal(t, schema.UserRoleID("net-2-network-admin"), roles[0].Slug)

		assert.NoError(t, (&schema.UserRole{Slug: "net-1-network-admin"}).GetNetworkRole(otherCtx),
			"deleting network roles is tenant scoped")

		require.NoError(t, (&schema.UserRole{Slug: "net-2-network-admin"}).DeleteNetworkRole(ctx))
		assert.ErrorIs(t, (&schema.UserRole{Slug: "net-2-network-admin"}).GetNetworkRole(ctx), gorm.ErrRecordNotFound)
	})
}
