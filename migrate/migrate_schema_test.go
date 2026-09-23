package migrate

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/orchestrator"
	"github.com/gravitl/netmaker/orchestrator/extensions"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMigrationTest(t *testing.T) context.Context {
	t.Helper()
	t.Chdir(t.TempDir())

	require.NoError(t, db.InitializeDB(schema.ListModels()...))
	t.Cleanup(db.CloseDB)

	orchestrator.InitializeRepository(extensions.NewCEFactory())

	ctx := db.WithContext(context.Background())
	require.NoError(t, ensureLegacyUserColumns(ctx))

	return ctx
}

func createKVTable(t *testing.T, ctx context.Context, tableName string) {
	t.Helper()
	err := db.FromContext(ctx).Exec(
		"CREATE TABLE IF NOT EXISTS " + tableName + " (key TEXT PRIMARY KEY, value TEXT NOT NULL)",
	).Error
	require.NoError(t, err)
}

func TestGetNetworkByNameForMigration(t *testing.T) {
	ctx := setupMigrationTest(t)

	network := &schema.Network{
		Name:         "orphan-net",
		AddressRange: "10.1.0.0/24",
	}
	require.NoError(t, network.Create(ctx))

	got, err := getNetworkByNameForMigration(ctx, "orphan-net")
	require.NoError(t, err)
	assert.Equal(t, network.ID, got.ID)
	assert.Empty(t, got.TenantID)

	_, err = getNetworkByNameForMigration(ctx, "missing-net")
	require.Error(t, err)
}

func TestToSQLSchema_RequiresV170(t *testing.T) {
	ctx := setupMigrationTest(t)

	admin := &schema.User{Username: "admin"}
	require.NoError(t, admin.Create(ctx))

	err := ToSQLSchema()
	require.ErrorIs(t, err, ErrMigrationV170Required)

	completed, err := migrationJobCompleted(ctx, migrationJobV180)
	require.NoError(t, err)
	assert.False(t, completed, "v1.8.0 migration must not run before v1.7.0 completes")
}

// TestMigrateV1_7_0_UsesSyncOrgAndTenantsHook ensures v1.7.0 step 0 goes through
// SyncOrgAndTenants (EE overrides this with license sync) rather than always
// calling CreateLocalDefaults, which breaks MSP installs that need multiple
// tenants from the account server.
func TestMigrateV1_7_0_UsesSyncOrgAndTenantsHook(t *testing.T) {
	ctx := setupMigrationTest(t)

	orig := SyncOrgAndTenants
	t.Cleanup(func() { SyncOrgAndTenants = orig })

	called := false
	SyncOrgAndTenants = func(ctx context.Context) error {
		called = true
		org := &schema.Organization{
			ID:   "license-org-id",
			Name: "msp-org",
		}
		require.NoError(t, org.Create(ctx))
		for _, tenantID := range []string{"tenant-a", "tenant-b"} {
			tenant := &schema.Tenant{
				ID:             tenantID,
				Name:           tenantID,
				OrganizationID: org.ID,
			}
			require.NoError(t, orchestrator.GetRepository().TenantOrchestrator().CreateTenant(ctx, tenant))
		}
		return nil
	}

	require.NoError(t, migrateV1_7_0(ctx))
	assert.True(t, called, "expected SyncOrgAndTenants hook to run")

	tenants, err := (&schema.Tenant{}).List(ctx)
	require.NoError(t, err)
	require.Len(t, tenants, 2)

	orgs, err := (&schema.Organization{}).ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, orgs, 1)
	assert.Equal(t, "license-org-id", orgs[0].ID)
}

func TestMigrateV1_6_0_ResolvesNetworkByNameWithoutTenant(t *testing.T) {
	ctx := setupMigrationTest(t)

	network := &schema.Network{
		Name:         "pre-mt-net",
		AddressRange: "10.2.0.0/24",
	}
	require.NoError(t, network.Create(ctx))
	assert.Empty(t, network.TenantID)

	createKVTable(t, ctx, TableName_Nodes)
	nodeID := uuid.New()
	hostID := uuid.New()
	require.NoError(t, kvInsert(ctx, TableName_Nodes, nodeID.String(), models.Node{
		CommonNode: models.CommonNode{
			ID:        nodeID,
			HostID:    hostID,
			Network:   network.Name,
			Connected: true,
		},
		LastModified: time.Now().UTC(),
		LastCheckIn:  time.Now().UTC(),
	}))

	require.NoError(t, migrateV1_6_0(ctx))

	node := &schema.Node{ID: nodeID.String()}
	require.NoError(t, node.Get(ctx))
	assert.Equal(t, network.ID, node.NetworkID)
}
