package migrate

import (
	"testing"

	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBackfillNetworkGlobalAccess(t *testing.T) {
	ctx := setupMigrationTest(t)
	tenantCtx := scope.WithContext(ctx, scope.TenantScope, "tenant-1")

	adminMissingFlag := &schema.UserRole{
		Slug:                "net-a-network-admin",
		Name:                "net-a Admin",
		Default:             true,
		NetworkID:           "net-a",
		NetworkGlobalAccess: false,
	}
	require.NoError(t, adminMissingFlag.Create(tenantCtx))

	globalAdmin := &schema.UserRole{
		Slug:                "global-network-admin",
		Name:                "Network Admins",
		Default:             true,
		NetworkID:           schema.AllNetworks,
		NetworkGlobalAccess: false,
	}
	require.NoError(t, globalAdmin.Create(tenantCtx))

	networkUser := &schema.UserRole{
		Slug:                "net-a-network-user",
		Name:                "net-a User",
		Default:             true,
		NetworkID:           "net-a",
		NetworkGlobalAccess: false,
	}
	require.NoError(t, networkUser.Create(tenantCtx))

	customAdminLooking := &schema.UserRole{
		Slug:                "custom-network-admin",
		Name:                "Custom",
		Default:             false,
		NetworkID:           "net-a",
		NetworkGlobalAccess: false,
	}
	require.NoError(t, customAdminLooking.Create(tenantCtx))

	alreadySet := &schema.UserRole{
		Slug:                "net-b-network-admin",
		Name:                "net-b Admin",
		Default:             true,
		NetworkID:           "net-b",
		NetworkGlobalAccess: true,
	}
	require.NoError(t, alreadySet.Create(tenantCtx))

	backfillNetworkGlobalAccess(tenantCtx)

	gotAdmin := &schema.UserRole{Slug: "net-a-network-admin"}
	require.NoError(t, gotAdmin.GetNetworkRole(tenantCtx))
	assert.True(t, gotAdmin.NetworkGlobalAccess)

	gotGlobal := &schema.UserRole{Slug: "global-network-admin"}
	require.NoError(t, gotGlobal.GetNetworkRole(tenantCtx))
	assert.True(t, gotGlobal.NetworkGlobalAccess)

	gotUser := &schema.UserRole{Slug: "net-a-network-user"}
	require.NoError(t, gotUser.GetNetworkRole(tenantCtx))
	assert.False(t, gotUser.NetworkGlobalAccess, "network-user must not be backfilled")

	gotCustom := &schema.UserRole{Slug: "custom-network-admin"}
	require.NoError(t, gotCustom.GetNetworkRole(tenantCtx))
	assert.False(t, gotCustom.NetworkGlobalAccess, "non-default roles must not be backfilled")

	gotAlready := &schema.UserRole{Slug: "net-b-network-admin"}
	require.NoError(t, gotAlready.GetNetworkRole(tenantCtx))
	assert.True(t, gotAlready.NetworkGlobalAccess)
}

func TestIsDefaultNetworkAdminRoleSlug(t *testing.T) {
	assert.True(t, isDefaultNetworkAdminRoleSlug(schema.UserRole{
		Default: true, NetworkID: "n", Slug: "n-network-admin",
	}))
	assert.True(t, isDefaultNetworkAdminRoleSlug(schema.UserRole{
		Default: true, NetworkID: schema.AllNetworks, Slug: "global-network-admin",
	}))
	assert.False(t, isDefaultNetworkAdminRoleSlug(schema.UserRole{
		Default: true, NetworkID: "n", Slug: "n-network-user",
	}))
	assert.False(t, isDefaultNetworkAdminRoleSlug(schema.UserRole{
		Default: false, NetworkID: "n", Slug: "n-network-admin",
	}))
}
