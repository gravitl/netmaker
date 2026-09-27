package schema_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	testutils "github.com/gravitl/netmaker/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func setupSchemaTest(t *testing.T) context.Context {
	t.Helper()
	t.Chdir(t.TempDir())

	require.NoError(t, db.InitializeDB(schema.ListModels()...))
	t.Cleanup(db.CloseDB)

	return tenantCtx("tenant-1")
}

func tenantCtx(tenantID string) context.Context {
	return scope.WithContext(db.WithContext(context.Background()), scope.TenantScope, tenantID)
}

func createExtclient(t *testing.T, ctx context.Context, network *schema.Network, name string) *schema.Extclient {
	t.Helper()

	extclient := &schema.Extclient{
		TenantID:  scope.ID(ctx),
		NetworkID: network.ID,
		Name:      name,
		Enabled:   true,
		Tags:      datatypes.NewJSONType(map[schema.TagID]struct{}{"tag-1": {}}),
	}
	require.NoError(t, extclient.Create(ctx))
	return extclient
}

func TestExtclient_Create(t *testing.T) {
	ctx := setupSchemaTest(t)
	network := testutils.CreateIPv4Network(t, ctx, "net-1")

	extclient := createExtclient(t, ctx, network, "client-1")
	_, err := uuid.Parse(extclient.ID)
	assert.NoError(t, err, "create generates a uuid id")

	t.Run("DoesNotCreateNetwork", func(t *testing.T) {
		extclient := &schema.Extclient{
			TenantID:  "tenant-1",
			NetworkID: network.ID,
			Network:   &schema.Network{ID: uuid.NewString(), Name: "should-not-exist"},
			Name:      "client-2",
		}
		require.NoError(t, extclient.Create(ctx))

		err := (&schema.Network{Name: "should-not-exist"}).Get(ctx)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})
}

func TestExtclient_Get(t *testing.T) {
	ctx := setupSchemaTest(t)
	network := testutils.CreateIPv4Network(t, ctx, "net-1")
	extclient := createExtclient(t, ctx, network, "client-1")

	t.Run("ByID", func(t *testing.T) {
		got := &schema.Extclient{ID: extclient.ID}
		require.NoError(t, got.Get(ctx))
		assert.Equal(t, "client-1", got.Name)
		assert.Equal(t, network.ID, got.NetworkID)
		assert.True(t, got.Enabled)
		assert.Contains(t, got.Tags.Data(), schema.TagID("tag-1"))
		assert.Nil(t, got.Network)
	})

	t.Run("ByNetworkIDAndName", func(t *testing.T) {
		got := &schema.Extclient{NetworkID: network.ID, Name: "client-1"}
		require.NoError(t, got.Get(ctx))
		assert.Equal(t, extclient.ID, got.ID)
	})

	t.Run("ByNetworkNameAndName", func(t *testing.T) {
		got := &schema.Extclient{Network: &schema.Network{Name: "net-1"}, Name: "client-1"}
		require.NoError(t, got.Get(ctx))
		assert.Equal(t, extclient.ID, got.ID)
		require.NotNil(t, got.Network)
		assert.Equal(t, network.ID, got.Network.ID)
		assert.Equal(t, "net-1", got.Network.Name)
	})

	t.Run("ByNetworkNameOfAnotherTenant", func(t *testing.T) {
		otherCtx := tenantCtx("tenant-2")
		other := testutils.CreateIPv4Network(t, otherCtx, "net-1")
		otherExtclient := createExtclient(t, otherCtx, other, "client-1")

		got := &schema.Extclient{Network: &schema.Network{Name: "net-1"}, Name: "client-1"}
		require.NoError(t, got.Get(otherCtx))
		assert.Equal(t, otherExtclient.ID, got.ID)
	})

	t.Run("ByNetworkNameRequiresTenant", func(t *testing.T) {
		got := &schema.Extclient{Network: &schema.Network{Name: "net-1"}, Name: "client-1"}
		err := got.Get(db.WithContext(context.Background()))
		assert.ErrorIs(t, err, schema.ErrExtclientIdentifiersNotProvided)
	})

	t.Run("ByIDInAnotherTenant", func(t *testing.T) {
		got := &schema.Extclient{ID: extclient.ID}
		err := got.Get(tenantCtx("tenant-2"))
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("ByIDWithoutTenant", func(t *testing.T) {
		got := &schema.Extclient{ID: extclient.ID}
		require.NoError(t, got.Get(db.WithContext(context.Background())))
		assert.Equal(t, "client-1", got.Name)
	})

	t.Run("NotFound", func(t *testing.T) {
		got := &schema.Extclient{NetworkID: network.ID, Name: "missing"}
		assert.ErrorIs(t, got.Get(ctx), gorm.ErrRecordNotFound)
	})

	t.Run("MissingIdentifiers", func(t *testing.T) {
		for _, extclient := range []*schema.Extclient{
			{},
			{Name: "client-1"},
			{NetworkID: network.ID},
			{Network: &schema.Network{Name: "net-1"}},
		} {
			assert.ErrorIs(t, extclient.Get(ctx), schema.ErrExtclientIdentifiersNotProvided)
		}
	})
}

func TestExtclient_Update(t *testing.T) {
	ctx := setupSchemaTest(t)
	network := testutils.CreateIPv4Network(t, ctx, "net-1")
	extclient := createExtclient(t, ctx, network, "client-1")

	createdAt := extclient.CreatedAt
	update := &schema.Extclient{
		ID:        extclient.ID,
		NetworkID: network.ID,
		Name:      "client-renamed",
		Enabled:   false,
		DNS:       "1.1.1.1",
	}
	require.NoError(t, update.Update(ctx))

	got := &schema.Extclient{ID: extclient.ID}
	require.NoError(t, got.Get(ctx))
	assert.Equal(t, "client-renamed", got.Name, "rename keeps the id")
	assert.False(t, got.Enabled, "zero values are written")
	assert.Equal(t, "1.1.1.1", got.DNS)
	assert.Empty(t, got.Tags.Data(), "all fields are overwritten")
	assert.Equal(t, "tenant-1", got.TenantID, "tenant is not overwritten")
	assert.WithinDuration(t, createdAt, got.CreatedAt, time.Second, "created at is not overwritten")

	assert.ErrorIs(t, (&schema.Extclient{Name: "client-renamed"}).Update(ctx), schema.ErrExtclientIdentifiersNotProvided)
}

func TestExtclient_Delete(t *testing.T) {
	ctx := setupSchemaTest(t)
	network := testutils.CreateIPv4Network(t, ctx, "net-1")

	byID := createExtclient(t, ctx, network, "client-1")
	require.NoError(t, (&schema.Extclient{ID: byID.ID}).Delete(ctx))
	assert.ErrorIs(t, (&schema.Extclient{ID: byID.ID}).Get(ctx), gorm.ErrRecordNotFound)

	createExtclient(t, ctx, network, "client-2")
	require.NoError(t, (&schema.Extclient{NetworkID: network.ID, Name: "client-2"}).Delete(ctx))
	assert.ErrorIs(t, (&schema.Extclient{NetworkID: network.ID, Name: "client-2"}).Get(ctx), gorm.ErrRecordNotFound)

	otherTenant := createExtclient(t, ctx, network, "client-3")
	require.NoError(t, (&schema.Extclient{ID: otherTenant.ID}).Delete(tenantCtx("tenant-2")))
	assert.NoError(t, (&schema.Extclient{ID: otherTenant.ID}).Get(ctx), "delete is tenant scoped")
}

func TestExtclient_ListByNetwork(t *testing.T) {
	ctx := setupSchemaTest(t)
	network := testutils.CreateIPv4Network(t, ctx, "net-1")
	other := testutils.CreateIPv4Network(t, ctx, "net-2")
	createExtclient(t, ctx, network, "client-1")
	createExtclient(t, ctx, network, "client-2")
	createExtclient(t, ctx, other, "client-3")

	byID, err := (&schema.Extclient{NetworkID: network.ID}).ListByNetwork(ctx)
	require.NoError(t, err)
	assert.Len(t, byID, 2)

	byName, err := (&schema.Extclient{Network: &schema.Network{Name: "net-1"}}).ListByNetwork(ctx)
	require.NoError(t, err)
	require.Len(t, byName, 2)
	for _, extclient := range byName {
		require.NotNil(t, extclient.Network)
		assert.Equal(t, "net-1", extclient.Network.Name)
	}

	count, err := (&schema.Extclient{Network: &schema.Network{Name: "net-1"}}).CountByNetwork(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	_, err = (&schema.Extclient{}).ListByNetwork(ctx)
	assert.ErrorIs(t, err, schema.ErrExtclientIdentifiersNotProvided)
}

func TestExtclient_DeleteByNetwork(t *testing.T) {
	ctx := setupSchemaTest(t)
	network := testutils.CreateIPv4Network(t, ctx, "net-1")
	other := testutils.CreateIPv4Network(t, ctx, "net-2")
	createExtclient(t, ctx, network, "client-1")
	createExtclient(t, ctx, network, "client-2")
	createExtclient(t, ctx, other, "client-3")

	require.NoError(t, (&schema.Extclient{NetworkID: network.ID}).DeleteByNetwork(ctx))

	count, err := (&schema.Extclient{}).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	assert.ErrorIs(t, (&schema.Extclient{}).DeleteByNetwork(ctx), schema.ErrExtclientIdentifiersNotProvided)
}

func TestExtclient_TenantScopedListCountAndDeleteAll(t *testing.T) {
	ctx := setupSchemaTest(t)
	otherCtx := tenantCtx("tenant-2")
	createExtclient(t, ctx, testutils.CreateIPv4Network(t, ctx, "net-1"), "client-1")
	createExtclient(t, otherCtx, testutils.CreateIPv4Network(t, otherCtx, "net-1"), "client-1")

	extclients, err := (&schema.Extclient{}).ListAll(ctx)
	require.NoError(t, err)
	assert.Len(t, extclients, 1)

	count, err := (&schema.Extclient{}).Count(db.WithContext(context.Background()))
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	require.NoError(t, (&schema.Extclient{}).DeleteAll(ctx))
	count, err = (&schema.Extclient{}).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
	count, err = (&schema.Extclient{}).Count(otherCtx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestExtclient_Violations(t *testing.T) {
	ctx := setupSchemaTest(t)
	network := testutils.CreateIPv4Network(t, ctx, "net-1")
	extclient := createExtclient(t, ctx, network, "client-1")

	extclient.PostureCheckSeverity = schema.SeverityHigh
	extclient.PostureCheckLastEvaluationCycleID = uuid.NewString()
	extclient.PostureCheckLastEvaluatedAt = time.Now().UTC()
	require.NoError(t, extclient.UpsertViolations(ctx, []schema.PostureCheckViolation{
		{CheckID: "check-1", Name: "os", Severity: schema.SeverityHigh},
	}))

	got := &schema.Extclient{ID: extclient.ID}
	require.NoError(t, got.Get(ctx))
	assert.Equal(t, schema.SeverityHigh, got.PostureCheckSeverity)
	violations, err := got.ListViolations(ctx)
	require.NoError(t, err)
	require.Len(t, violations, 1)
	assert.Equal(t, schema.PostureCheckSubjectTypeExtclient, violations[0].SubjectType)
	assert.Equal(t, extclient.ID, violations[0].SubjectID)
	assert.Equal(t, "tenant-1", violations[0].TenantID)

	// a new cycle replaces the previous one.
	extclient.PostureCheckLastEvaluationCycleID = uuid.NewString()
	require.NoError(t, extclient.UpsertViolations(ctx, []schema.PostureCheckViolation{
		{CheckID: "check-2", Name: "version", Severity: schema.SeverityHigh},
	}))
	require.NoError(t, got.Get(ctx))
	violations, err = got.ListViolations(ctx)
	require.NoError(t, err)
	require.Len(t, violations, 1)
	assert.Equal(t, "check-2", violations[0].CheckID)

	require.NoError(t, extclient.DeleteViolations(ctx))
	violations, err = got.ListViolations(ctx)
	require.NoError(t, err)
	assert.Empty(t, violations)
}
