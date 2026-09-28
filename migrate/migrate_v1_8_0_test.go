package migrate

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/migrate/types"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func createMigrationTestNetwork(t *testing.T, ctx context.Context, tenantID, name string) *schema.Network {
	t.Helper()

	network := &schema.Network{
		TenantID:     tenantID,
		Name:         name,
		AddressRange: "10.0.0.0/24",
	}
	require.NoError(t, network.Create(ctx))
	return network
}

func seedExtClientRecord(t *testing.T, ctx context.Context, tenantID string, extclient types.ExtClient) {
	t.Helper()

	record := &types.ExtClientRecord{
		Key:      tenantID + "::" + extclient.ClientID + "###" + extclient.Network,
		TenantID: tenantID,
		Value:    datatypes.NewJSONType(extclient),
	}
	require.NoError(t, db.FromContext(ctx).Create(record).Error)
}

func legacyExtClient(name, network string) types.ExtClient {
	jitExpiresAt := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	return types.ExtClient{
		ClientID:                 name,
		PrivateKey:               "private-key",
		PublicKey:                "public-key",
		Network:                  network,
		DNS:                      "1.1.1.1",
		Address:                  "10.0.0.10",
		Address6:                 "fd00::10",
		ExtraAllowedIPs:          []string{"192.168.1.0/24"},
		AllowedIPs:               []string{"10.0.0.0/24"},
		IngressGatewayID:         "gateway-id",
		IngressGatewayEndpoint:   "1.2.3.4:51821",
		SelectedInternetEgressID: "egress-id",
		LastModified:             time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).Unix(),
		Enabled:                  true,
		OwnerID:                  "owner",
		DeniedACLs:               map[string]struct{}{"denied": {}},
		RemoteAccessClientID:     "rac-id",
		PostUp:                   "up",
		PostDown:                 "down",
		Tags:                     map[schema.TagID]struct{}{"tag-1": {}},
		OS:                       "linux",
		OSFamily:                 "debian",
		OSVersion:                "12",
		KernelVersion:            "6.1",
		ClientVersion:            "v1.7.0",
		DeviceID:                 "device-id",
		DeviceName:               "device-name",
		PublicEndpoint:           "5.6.7.8",
		Country:                  "IN",
		Location:                 "1,2",
		JITExpiresAt:             &jitExpiresAt,
		Status:                   schema.OnlineSt,
	}
}

func setupExtClientMigrationTest(t *testing.T) context.Context {
	t.Helper()

	ctx := setupMigrationTest(t)
	require.NoError(t, db.FromContext(ctx).AutoMigrate(&types.ExtClientRecord{}))
	return ctx
}

func TestMigrateExtClients(t *testing.T) {
	ctx := setupExtClientMigrationTest(t)
	network := createMigrationTestNetwork(t, ctx, "tenant-1", "net-1")
	otherTenantNetwork := createMigrationTestNetwork(t, ctx, "tenant-2", "net-1")

	evaluatedAt := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	withViolations := legacyExtClient("client-1", "net-1")
	withViolations.PostureCheckVolationSeverityLevel = schema.SeverityHigh
	withViolations.LastEvaluatedAt = evaluatedAt
	withViolations.PostureChecksViolations = []types.Violation{
		{CheckID: "check-1", Name: "os", Attribute: "os", Message: "bad os", Severity: schema.SeverityHigh},
		{CheckID: "check-2", Name: "version", Attribute: "version", Message: "old", Severity: schema.SeverityLow},
	}
	seedExtClientRecord(t, ctx, "tenant-1", withViolations)
	seedExtClientRecord(t, ctx, "tenant-1", legacyExtClient("client-2", "net-1"))
	// same client and network names, in another tenant.
	seedExtClientRecord(t, ctx, "tenant-2", legacyExtClient("client-1", "net-1"))

	require.NoError(t, migrateExtClients(ctx))

	count, err := (&schema.Extclient{}).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	tenantCtx := scope.WithContext(ctx, scope.TenantScope, "tenant-1")
	_extclient := &schema.Extclient{NetworkID: network.ID, Name: "client-1"}
	require.NoError(t, _extclient.Get(tenantCtx))
	_, err = uuid.Parse(_extclient.ID)
	assert.NoError(t, err)
	assert.Equal(t, "tenant-1", _extclient.TenantID)
	assert.True(t, time.Unix(withViolations.LastModified, 0).Equal(_extclient.CreatedAt))
	assert.True(t, time.Unix(withViolations.LastModified, 0).Equal(_extclient.UpdatedAt))
	assert.NotEmpty(t, _extclient.PostureCheckLastEvaluationCycleID)

	// the migrated extclient reads back as it was stored.
	got, err := logic.GetExtClient(tenantCtx, "client-1", "net-1")
	require.NoError(t, err)
	assert.Equal(t, _extclient.ID, got.ID)
	assert.Equal(t, withViolations.PostureCheckVolationSeverityLevel, got.PostureCheckVolationSeverityLevel)
	assert.True(t, evaluatedAt.Equal(got.LastEvaluatedAt))
	require.NotNil(t, got.JITExpiresAt)
	assert.True(t, withViolations.JITExpiresAt.Equal(*got.JITExpiresAt))
	assert.ElementsMatch(t, []models.Violation{
		{CheckID: "check-1", Name: "os", Attribute: "os", Message: "bad os", Severity: schema.SeverityHigh},
		{CheckID: "check-2", Name: "version", Attribute: "version", Message: "old", Severity: schema.SeverityLow},
	}, got.PostureChecksViolations)

	want := models.ExtClient{
		ID:                                got.ID,
		TenantID:                          "tenant-1",
		ClientID:                          withViolations.ClientID,
		PrivateKey:                        withViolations.PrivateKey,
		PublicKey:                         withViolations.PublicKey,
		Network:                           withViolations.Network,
		DNS:                               withViolations.DNS,
		Address:                           withViolations.Address,
		Address6:                          withViolations.Address6,
		ExtraAllowedIPs:                   withViolations.ExtraAllowedIPs,
		AllowedIPs:                        withViolations.AllowedIPs,
		IngressGatewayID:                  withViolations.IngressGatewayID,
		IngressGatewayEndpoint:            withViolations.IngressGatewayEndpoint,
		SelectedInternetEgressID:          withViolations.SelectedInternetEgressID,
		LastModified:                      withViolations.LastModified,
		Enabled:                           withViolations.Enabled,
		OwnerID:                           withViolations.OwnerID,
		DeniedACLs:                        withViolations.DeniedACLs,
		RemoteAccessClientID:              withViolations.RemoteAccessClientID,
		PostUp:                            withViolations.PostUp,
		PostDown:                          withViolations.PostDown,
		Tags:                              withViolations.Tags,
		OS:                                withViolations.OS,
		OSFamily:                          withViolations.OSFamily,
		OSVersion:                         withViolations.OSVersion,
		KernelVersion:                     withViolations.KernelVersion,
		ClientVersion:                     withViolations.ClientVersion,
		DeviceID:                          withViolations.DeviceID,
		DeviceName:                        withViolations.DeviceName,
		PublicEndpoint:                    withViolations.PublicEndpoint,
		Country:                           withViolations.Country,
		Location:                          withViolations.Location,
		PostureCheckVolationSeverityLevel: withViolations.PostureCheckVolationSeverityLevel,
		LastEvaluationCycleID:             _extclient.PostureCheckLastEvaluationCycleID,
		Status:                            withViolations.Status,
	}
	compared := got
	compared.PostureChecksViolations, compared.LastEvaluatedAt, compared.JITExpiresAt, compared.Mutex = nil, time.Time{}, nil, nil
	assert.Equal(t, want, compared)

	t.Run("NoViolations", func(t *testing.T) {
		got, err := logic.GetExtClient(tenantCtx, "client-2", "net-1")
		require.NoError(t, err)
		assert.Empty(t, got.PostureChecksViolations)
		assert.Empty(t, got.LastEvaluationCycleID)
	})

	t.Run("ResolvesNetworkInTenant", func(t *testing.T) {
		otherCtx := scope.WithContext(ctx, scope.TenantScope, "tenant-2")
		other := &schema.Extclient{NetworkID: otherTenantNetwork.ID, Name: "client-1"}
		require.NoError(t, other.Get(otherCtx))
		assert.Equal(t, "tenant-2", other.TenantID)
		assert.NotEqual(t, _extclient.ID, other.ID)
	})

	t.Run("KeepsOldTable", func(t *testing.T) {
		var records int64
		require.NoError(t, db.FromContext(ctx).Model(&types.ExtClientRecord{}).Count(&records).Error)
		assert.Equal(t, int64(3), records)
	})
}

func TestMigrateExtClients_SkipsUnmigratableRecords(t *testing.T) {
	ctx := setupExtClientMigrationTest(t)
	network := createMigrationTestNetwork(t, ctx, "tenant-1", "net-1")

	seedExtClientRecord(t, ctx, "tenant-1", legacyExtClient("client-1", "net-1"))
	// network deleted.
	seedExtClientRecord(t, ctx, "tenant-1", legacyExtClient("client-2", "deleted-net"))
	// network of another tenant.
	seedExtClientRecord(t, ctx, "tenant-2", legacyExtClient("client-3", "net-1"))
	// no client id.
	seedExtClientRecord(t, ctx, "tenant-1", legacyExtClient("", "net-1"))

	require.NoError(t, migrateExtClients(ctx))

	extclients, err := (&schema.Extclient{}).ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, extclients, 1)
	assert.Equal(t, "client-1", extclients[0].Name)
	assert.Equal(t, network.ID, extclients[0].NetworkID)
}

func TestMigrateExtClients_NoOldTable(t *testing.T) {
	ctx := setupMigrationTest(t)
	require.False(t, db.FromContext(ctx).Migrator().HasTable(&types.ExtClientRecord{}))

	require.NoError(t, migrateExtClients(ctx))

	count, err := (&schema.Extclient{}).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestMigrateExtClients_RollsBackWithMigrationJob(t *testing.T) {
	ctx := setupExtClientMigrationTest(t)
	createMigrationTestNetwork(t, ctx, "tenant-1", "net-1")
	seedExtClientRecord(t, ctx, "tenant-1", legacyExtClient("client-1", "net-1"))

	err := ensureMigrationCompleted(ctx, "test-extclients-rollback", func(ctx context.Context) error {
		if err := migrateExtClients(ctx); err != nil {
			return err
		}
		return gorm.ErrInvalidData
	})
	require.ErrorIs(t, err, gorm.ErrInvalidData)

	count, err := (&schema.Extclient{}).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "a failed migration job leaves no migrated extclients")
}
