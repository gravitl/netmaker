package migrate

import (
	"bytes"
	"context"
	"log"
	"testing"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/orchestrator"
	"github.com/gravitl/netmaker/orchestrator/extensions"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	gormlogger "gorm.io/gorm/logger"
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

func TestMigrateEgressDomains_SkipsMissingLegacyColumn(t *testing.T) {
	ctx := setupMigrationTest(t)
	gormDB := db.FromContext(ctx)
	require.False(t, gormDB.Migrator().HasColumn((&schema.Egress{}).Table(), "domain"))
	for _, id := range []string{"first-egress", "second-egress"} {
		require.NoError(t, (&schema.Egress{ID: id}).Create(ctx))
	}

	var sqlLog bytes.Buffer
	originalLogger := gormDB.Logger
	gormDB.Logger = gormlogger.New(log.New(&sqlLog, "", 0), gormlogger.Config{LogLevel: gormlogger.Info})
	t.Cleanup(func() { gormDB.Logger = originalLogger })

	migrateEgressDomains()

	assert.NotContains(t, sqlLog.String(), "SELECT domain", "fresh schemas must skip legacy domain queries")
	assert.NotContains(t, sqlLog.String(), "no such column", "fresh schemas must not produce missing-column errors")
}

func TestMigrateEgressDomains_CopiesLegacyDomain(t *testing.T) {
	ctx := setupMigrationTest(t)
	gormDB := db.FromContext(ctx)
	require.NoError(t, gormDB.Exec("ALTER TABLE egresses ADD COLUMN domain TEXT").Error)

	for _, tt := range []struct {
		id      string
		domains datatypes.JSONSlice[string]
		want    datatypes.JSONSlice[string]
	}{
		{id: "legacy-egress", want: datatypes.JSONSlice[string]{"example.com"}},
		{id: "migrated-egress", domains: datatypes.JSONSlice[string]{"current.example.com"}, want: datatypes.JSONSlice[string]{"current.example.com"}},
	} {
		t.Run(tt.id, func(t *testing.T) {
			egress := &schema.Egress{ID: tt.id, Domains: tt.domains}
			require.NoError(t, egress.Create(ctx))
			require.NoError(t, gormDB.Table(egress.Table()).Where("id = ?", egress.ID).Update("domain", "  EXAMPLE.COM  ").Error)

			migrateEgressDomains()

			got := &schema.Egress{ID: egress.ID}
			require.NoError(t, got.Get(ctx))
			assert.Equal(t, tt.want, got.Domains)
		})
	}
}
