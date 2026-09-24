package migrate

import (
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertIsUUID(t *testing.T, id string) {
	t.Helper()
	_, err := uuid.Parse(id)
	assert.NoError(t, err, "expected a fresh uuid id, got %q", id)
}

func TestMigrateUserRolesToScoped(t *testing.T) {
	ctx := setupMigrationTest(t)

	platformRow := schema.UserRole{ID: "super-admin", Default: true, TenantGlobalAccess: true}
	require.NoError(t, db.FromContext(ctx).Model(&schema.UserRole{}).Create(&platformRow).Error)

	orgRow := schema.UserRole{ID: "org-owner", Default: true, OrgGlobalAccess: true}
	require.NoError(t, db.FromContext(ctx).Model(&schema.UserRole{}).Create(&orgRow).Error)

	networkRow := schema.UserRole{
		ID:        schema.UserRoleID(schema.TenantScopedKey("tenant-a", "net1-network-admin")),
		Default:   true,
		NetworkID: "net1",
	}
	require.NoError(t, db.FromContext(ctx).Model(&schema.UserRole{}).Create(&networkRow).Error)

	require.NoError(t, migrateUserRolesToScoped(ctx))

	var platformRole schema.UserRole
	require.NoError(t, db.FromContext(ctx).Model(&schema.UserRole{}).Where("name = ?", "super-admin").First(&platformRole).Error)
	assertIsUUID(t, platformRole.ID.String())
	assert.Equal(t, scope.TenantScope, platformRole.Scope)
	assert.Empty(t, platformRole.ScopeID)

	var orgRole schema.UserRole
	require.NoError(t, db.FromContext(ctx).Model(&schema.UserRole{}).Where("name = ?", "org-owner").First(&orgRole).Error)
	assertIsUUID(t, orgRole.ID.String())
	assert.Equal(t, scope.OrgScope, orgRole.Scope)
	assert.Empty(t, orgRole.ScopeID)

	var networkRole schema.UserRole
	require.NoError(t, db.FromContext(ctx).Model(&schema.UserRole{}).Where("network_id <> ''").First(&networkRole).Error)
	assertIsUUID(t, networkRole.ID.String())
	assert.Equal(t, "net1-network-admin", networkRole.Name)
	assert.Equal(t, scope.TenantScope, networkRole.Scope)
	assert.Equal(t, "tenant-a", networkRole.ScopeID)

	// Idempotency: running it again on an already-migrated DB is a no-op -
	// none of the rows matches the "::" prefix or "Default && Name == \"\""
	// gate a second time.
	require.NoError(t, migrateUserRolesToScoped(ctx))

	var count int64
	require.NoError(t, db.FromContext(ctx).Model(&schema.UserRole{}).Count(&count).Error)
	assert.EqualValues(t, 3, count, "migration must not duplicate rows on a second run")
}
