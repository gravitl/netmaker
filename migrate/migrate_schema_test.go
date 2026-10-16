package migrate

import (
	"context"
	"testing"

	"github.com/gravitl/netmaker/db"
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
