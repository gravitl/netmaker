package migrate

import (
	"testing"

	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestRekeyTenant(t *testing.T) {
	ctx := setupMigrationTest(t)
	oldCtx := scope.WithContext(ctx, scope.TenantScope, "tenant-old")
	newCtx := scope.WithContext(ctx, scope.TenantScope, "tenant-new")

	require.NoError(t, (&schema.UserRole{Slug: schema.SuperAdminRole}).Upsert(ctx))
	require.NoError(t, (&schema.UserRole{Slug: "net-1-network-admin", NetworkID: "net-1"}).Upsert(oldCtx))
	require.NoError(t, (&schema.UserGroup{TenantID: "tenant-old", Slug: "net-1-network-admin-grp"}).Create(oldCtx))
	require.NoError(t, (&schema.AclRecord{Slug: "net-1.all-nodes", Value: datatypes.NewJSONType(schema.Acl{})}).Upsert(oldCtx))

	network := &schema.Network{TenantID: "tenant-old", Name: "net-1", AddressRange: "10.0.0.0/24"}
	require.NoError(t, network.Create(oldCtx))
	require.NoError(t, (&schema.Tag{TenantID: "tenant-old", NetworkID: network.ID, Name: "tag-1"}).Create(oldCtx))
	require.NoError(t, (&schema.DNSEntry{TenantID: "tenant-old", NetworkID: network.ID, Name: "host-1"}).Create(oldCtx))

	require.NoError(t, RekeyTenant(ctx, "tenant-old", "tenant-new"))

	role := &schema.UserRole{Slug: "net-1-network-admin"}
	require.NoError(t, role.GetNetworkRole(newCtx))
	assert.Equal(t, "tenant-new", role.ScopeID)

	platformRole := &schema.UserRole{Slug: schema.SuperAdminRole}
	require.NoError(t, platformRole.GetPlatformRole(ctx))
	assert.Equal(t, scope.GlobalScope, platformRole.Scope, "global roles are not rekeyed")
	assert.Empty(t, platformRole.ScopeID)

	group := &schema.UserGroup{Slug: "net-1-network-admin-grp"}
	require.NoError(t, group.Get(newCtx))

	acl := &schema.AclRecord{Slug: "net-1.all-nodes"}
	require.NoError(t, acl.Get(newCtx))

	tag := &schema.Tag{Network: &schema.Network{Name: "net-1"}, Name: "tag-1"}
	require.NoError(t, tag.Get(newCtx))

	entry := &schema.DNSEntry{Network: &schema.Network{Name: "net-1"}, Name: "host-1"}
	require.NoError(t, entry.Get(newCtx))

	for _, count := range []func() (int, error){
		func() (int, error) { return (&schema.UserGroup{}).Count(oldCtx) },
		func() (int, error) { return (&schema.AclRecord{}).Count(oldCtx) },
		func() (int, error) { return (&schema.Tag{}).Count(oldCtx) },
		func() (int, error) { return (&schema.DNSEntry{}).Count(oldCtx) },
	} {
		n, err := count()
		require.NoError(t, err)
		assert.Zero(t, n, "nothing is left in the old tenant")
	}
}
