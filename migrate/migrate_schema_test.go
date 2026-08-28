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
	"gorm.io/datatypes"
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

func markMigrationJobComplete(t *testing.T, ctx context.Context, jobID string) {
	t.Helper()
	job := &schema.Job{ID: jobID}
	require.NoError(t, job.Create(ctx))
}

func assertMigrationJobComplete(t *testing.T, ctx context.Context, jobID string) {
	t.Helper()
	completed, err := migrationJobCompleted(ctx, jobID)
	require.NoError(t, err)
	assert.True(t, completed, "expected migration job %s to be complete", jobID)
}

func seedLegacyKVData(t *testing.T, ctx context.Context) (networkName string, nodeID uuid.UUID) {
	t.Helper()

	createKVTable(t, ctx, TableName_Users)
	createKVTable(t, ctx, TableName_Networks)
	createKVTable(t, ctx, TableName_Nodes)
	createKVTable(t, ctx, TableName_Hosts)

	networkName = "testnet"
	nodeID = uuid.New()
	hostID := uuid.New()

	require.NoError(t, kvInsert(ctx, TableName_Users, "admin", models.User{
		UserName:     "admin",
		Password:     "secret",
		IsSuperAdmin: true,
		AuthType:     schema.BasicAuth,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}))

	require.NoError(t, kvInsert(ctx, TableName_Networks, networkName, models.Network{
		NetID:        networkName,
		AddressRange: "10.0.0.0/24",
		CreatedAt:    time.Now().UTC(),
	}))

	require.NoError(t, kvInsert(ctx, TableName_Hosts, hostID.String(), models.Host{
		ID:   hostID,
		Name: "test-host",
		OS:   models.OS_Types.Linux,
	}))

	require.NoError(t, kvInsert(ctx, TableName_Nodes, nodeID.String(), models.Node{
		CommonNode: models.CommonNode{
			ID:        nodeID,
			HostID:    hostID,
			Network:   networkName,
			Connected: true,
		},
		LastModified: time.Now().UTC(),
		LastCheckIn:  time.Now().UTC(),
	}))

	return networkName, nodeID
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

func TestToSQLSchema_FullKVUpgrade(t *testing.T) {
	ctx := setupMigrationTest(t)
	networkName, nodeID := seedLegacyKVData(t, ctx)

	require.NoError(t, ToSQLSchema())

	assertMigrationJobComplete(t, ctx, "migration-v1.5.1")
	assertMigrationJobComplete(t, ctx, "migration-v1.6.0")
	assertMigrationJobComplete(t, ctx, "migration-v1.7.0")

	tenants, err := (&schema.Tenant{}).List(ctx)
	require.NoError(t, err)
	require.Len(t, tenants, 1)

	node := &schema.Node{ID: nodeID.String()}
	require.NoError(t, node.Get(ctx))
	assert.NotEmpty(t, node.NetworkID)

	networks, err := (&schema.Network{}).ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, networks, 1)
	assert.Equal(t, networkName, networks[0].Name)
	assert.Equal(t, tenants[0].ID, networks[0].TenantID)

	var membershipCount int64
	require.NoError(t, db.FromContext(ctx).Model(&schema.TenantMembership{}).Count(&membershipCount).Error)
	assert.Equal(t, int64(1), membershipCount)
}

func TestToSQLSchema_StuckServerCompletesV160AndV170(t *testing.T) {
	ctx := setupMigrationTest(t)
	networkName, nodeID := seedLegacyKVData(t, ctx)

	require.NoError(t, CreateLocalDefaults(ctx))
	markMigrationJobComplete(t, ctx, "migration-multitenancy")
	markMigrationJobComplete(t, ctx, "migration-v1.5.1")

	// Simulate v1.5.1 output: SQL rows exist without tenant_id.
	require.NoError(t, db.FromContext(ctx).Exec("DELETE FROM users_v1").Error)
	require.NoError(t, db.FromContext(ctx).Exec("DELETE FROM networks_v1").Error)
	require.NoError(t, db.FromContext(ctx).Exec("DELETE FROM hosts_v1").Error)

	admin := &schema.User{Username: "admin", DisplayName: "Admin"}
	require.NoError(t, admin.Create(ctx))
	require.NoError(t, upsertLegacyUserAuth(ctx, admin.ID, models.User{
		UserName:     "admin",
		Password:     "secret",
		IsSuperAdmin: true,
		AuthType:     schema.BasicAuth,
	}, schema.SuperAdminRole, datatypesNewEmptyGroups()))

	network := &schema.Network{
		Name:         networkName,
		AddressRange: "10.0.0.0/24",
	}
	require.NoError(t, network.Create(ctx))

	require.NoError(t, ToSQLSchema())

	assertMigrationJobComplete(t, ctx, "migration-v1.6.0")
	assertMigrationJobComplete(t, ctx, "migration-v1.7.0")

	node := &schema.Node{ID: nodeID.String()}
	require.NoError(t, node.Get(ctx))
	assert.Equal(t, network.ID, node.NetworkID)
}

func TestToSQLSchema_SkipsCompletedPreMTJobs(t *testing.T) {
	ctx := setupMigrationTest(t)

	require.NoError(t, CreateLocalDefaults(ctx))
	markMigrationJobComplete(t, ctx, "migration-multitenancy")
	markMigrationJobComplete(t, ctx, "migration-v1.5.1")
	markMigrationJobComplete(t, ctx, "migration-v1.6.0")

	admin := &schema.User{Username: "admin"}
	require.NoError(t, admin.Create(ctx))
	require.NoError(t, upsertLegacyUserAuth(ctx, admin.ID, models.User{
		UserName: "admin",
		AuthType: schema.BasicAuth,
	}, schema.SuperAdminRole, datatypesNewEmptyGroups()))

	require.NoError(t, ToSQLSchema())

	assertMigrationJobComplete(t, ctx, "migration-v1.7.0")

	tenants, err := (&schema.Tenant{}).List(ctx)
	require.NoError(t, err)
	require.Len(t, tenants, 1)
}

func TestToSQLSchema_IdempotentRerun(t *testing.T) {
	ctx := setupMigrationTest(t)
	seedLegacyKVData(t, ctx)

	require.NoError(t, ToSQLSchema())

	var firstMembershipCount int64
	require.NoError(t, db.FromContext(ctx).Model(&schema.TenantMembership{}).Count(&firstMembershipCount).Error)
	require.Equal(t, int64(1), firstMembershipCount)

	require.NoError(t, ToSQLSchema())

	var secondMembershipCount int64
	require.NoError(t, db.FromContext(ctx).Model(&schema.TenantMembership{}).Count(&secondMembershipCount).Error)
	assert.Equal(t, firstMembershipCount, secondMembershipCount)
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

func datatypesNewEmptyGroups() datatypes.JSONType[map[schema.UserGroupID]struct{}] {
	return datatypes.NewJSONType(make(map[schema.UserGroupID]struct{}))
}
