package logic

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func extClientTestCtx(tenantID string) context.Context {
	return scope.WithContext(db.WithContext(context.Background()), scope.TenantScope, tenantID)
}

func newTestExtClient(name, network string) *models.ExtClient {
	jitExpiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	return &models.ExtClient{
		ClientID:                 name,
		PrivateKey:               "private-key",
		PublicKey:                "public-key",
		Network:                  network,
		DNS:                      "1.1.1.1",
		Address:                  "10.0.0.10",
		Address6:                 "fd00::10",
		ExtraAllowedIPs:          []string{"192.168.1.0/24"},
		AllowedIPs:               []string{"10.0.0.0/24"},
		IngressGatewayID:         uuid.NewString(),
		IngressGatewayEndpoint:   "1.2.3.4:51821",
		SelectedInternetEgressID: uuid.NewString(),
		Enabled:                  true,
		OwnerID:                  "owner",
		DeniedACLs:               map[string]struct{}{"denied": {}},
		PostUp:                   "up",
		PostDown:                 "down",
		Tags:                     map[models.TagID]struct{}{"tag-1": {}},
		OS:                       "linux",
		OSFamily:                 "debian",
		OSVersion:                "12",
		KernelVersion:            "6.1",
		ClientVersion:            "v1.8.0",
		DeviceID:                 "device-id",
		DeviceName:               "device-name",
		PublicEndpoint:           "5.6.7.8",
		Country:                  "IN",
		Location:                 "1,2",
		JITExpiresAt:             &jitExpiresAt,
		Status:                   schema.OnlineSt,
	}
}

func TestSaveExtClient(t *testing.T) {
	ctx := extClientTestCtx("tenant-1")
	network := createExtClientTestNetwork(t, ctx, "extclient-save-net")

	extclient := newTestExtClient("client-save", network.Name)
	require.NoError(t, SaveExtClient(ctx, extclient))
	_, err := uuid.Parse(extclient.ID)
	require.NoError(t, err, "create sets the id")
	assert.Equal(t, "tenant-1", extclient.TenantID, "tenant defaults to the scope")

	got, err := GetExtClient(ctx, "client-save", network.Name)
	require.NoError(t, err)
	assert.NotZero(t, got.LastModified)
	assert.NotNil(t, got.Mutex)
	require.NotNil(t, got.JITExpiresAt)
	assert.True(t, extclient.JITExpiresAt.Equal(*got.JITExpiresAt))
	assert.True(t, got.LastEvaluatedAt.IsZero())
	want := *extclient
	want.JITExpiresAt = nil
	compared := got
	compared.LastModified, compared.Mutex, compared.JITExpiresAt, compared.LastEvaluatedAt = 0, nil, nil, time.Time{}
	assert.Equal(t, want, compared)

	t.Run("Rename", func(t *testing.T) {
		got.ClientID = "client-renamed"
		got.Enabled = false
		require.NoError(t, SaveExtClient(ctx, &got))

		renamed, err := GetExtClient(ctx, "client-renamed", network.Name)
		require.NoError(t, err)
		assert.Equal(t, extclient.ID, renamed.ID, "rename keeps the id")
		assert.False(t, renamed.Enabled)

		_, err = GetExtClient(ctx, "client-save", network.Name)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("UpdateMissing", func(t *testing.T) {
		missing := newTestExtClient("client-missing", network.Name)
		missing.ID = uuid.NewString()
		assert.ErrorIs(t, SaveExtClient(ctx, missing), gorm.ErrRecordNotFound)
	})

	t.Run("MissingNetwork", func(t *testing.T) {
		assert.ErrorIs(t, SaveExtClient(ctx, newTestExtClient("client-orphan", "missing-net")), gorm.ErrRecordNotFound)
	})
}

func TestSaveExtClient_KeepsPostureCheckFields(t *testing.T) {
	ctx := extClientTestCtx("tenant-1")
	network := createExtClientTestNetwork(t, ctx, "extclient-posture-net")

	extclient := newTestExtClient("client-posture", network.Name)
	require.NoError(t, SaveExtClient(ctx, extclient))

	_extclient := &schema.Extclient{
		ID:                                extclient.ID,
		TenantID:                          extclient.TenantID,
		PostureCheckSeverity:              schema.SeverityHigh,
		PostureCheckLastEvaluationCycleID: uuid.NewString(),
		PostureCheckLastEvaluatedAt:       time.Now().UTC(),
	}
	require.NoError(t, _extclient.UpsertViolations(ctx, []schema.PostureCheckViolation{
		{CheckID: "check-1", Name: "os", Severity: schema.SeverityHigh},
	}))

	// a stale model does not overwrite the evaluation.
	require.NoError(t, SaveExtClient(ctx, extclient))

	got, err := GetExtClient(ctx, "client-posture", network.Name)
	require.NoError(t, err)
	assert.Equal(t, schema.SeverityHigh, got.PostureCheckVolationSeverityLevel)
	assert.Equal(t, _extclient.PostureCheckLastEvaluationCycleID, got.LastEvaluationCycleID)
	require.Len(t, got.PostureChecksViolations, 1)
	assert.Equal(t, "check-1", got.PostureChecksViolations[0].CheckID)

	extclients, err := GetNetworkExtClients(ctx, network.Name)
	require.NoError(t, err)
	require.Len(t, extclients, 1)
	assert.Empty(t, extclients[0].PostureChecksViolations, "lists skip violations")
	assert.Equal(t, schema.SeverityHigh, extclients[0].PostureCheckVolationSeverityLevel)
}

func TestGetNetworkAndAllExtClients(t *testing.T) {
	ctx := extClientTestCtx("tenant-list")
	otherCtx := extClientTestCtx("tenant-list-other")
	network := createExtClientTestNetwork(t, ctx, "extclient-list-net")
	other := createExtClientTestNetwork(t, ctx, "extclient-list-net-2")
	otherTenantNetwork := createExtClientTestNetwork(t, otherCtx, "extclient-list-net")

	require.NoError(t, SaveExtClient(ctx, newTestExtClient("client-1", network.Name)))
	require.NoError(t, SaveExtClient(ctx, newTestExtClient("client-2", network.Name)))
	require.NoError(t, SaveExtClient(ctx, newTestExtClient("client-3", other.Name)))
	require.NoError(t, SaveExtClient(otherCtx, newTestExtClient("client-1", otherTenantNetwork.Name)))

	extclients, err := GetNetworkExtClients(ctx, network.Name)
	require.NoError(t, err)
	require.Len(t, extclients, 2)
	for _, extclient := range extclients {
		assert.Equal(t, network.Name, extclient.Network)
		assert.Equal(t, "tenant-list", extclient.TenantID)
	}

	all, err := GetAllExtClients(ctx)
	require.NoError(t, err)
	require.Len(t, all, 3)
	networks := map[string]int{}
	for _, extclient := range all {
		networks[extclient.Network]++
	}
	assert.Equal(t, map[string]int{network.Name: 2, other.Name: 1}, networks)

	all, err = GetAllExtClients(otherCtx)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, "tenant-list-other", all[0].TenantID)
}

func TestDeleteExtClient(t *testing.T) {
	ctx := extClientTestCtx("tenant-1")
	network := createExtClientTestNetwork(t, ctx, "extclient-delete-net")

	extclient := newTestExtClient("client-delete", network.Name)
	extclient.DeviceID = ""
	require.NoError(t, SaveExtClient(ctx, extclient))

	_extclient := &schema.Extclient{
		ID:                                extclient.ID,
		TenantID:                          extclient.TenantID,
		PostureCheckSeverity:              schema.SeverityHigh,
		PostureCheckLastEvaluationCycleID: uuid.NewString(),
	}
	require.NoError(t, _extclient.UpsertViolations(ctx, []schema.PostureCheckViolation{
		{CheckID: "check-1", Severity: schema.SeverityHigh},
	}))

	require.NoError(t, DeleteExtClient(ctx, *extclient))

	_, err := GetExtClient(ctx, "client-delete", network.Name)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	violations, err := _extclient.ListViolations(ctx)
	require.NoError(t, err)
	assert.Empty(t, violations)

	assert.ErrorIs(t, DeleteExtClient(ctx, *extclient), gorm.ErrRecordNotFound, "deleting a deleted ext client fails")
}

func TestConvertExtClient(t *testing.T) {
	ctx := extClientTestCtx("tenant-1")
	network := createExtClientTestNetwork(t, ctx, "extclient-convert-net")

	extclient := newTestExtClient("client-convert", network.Name)
	extclient.ID = uuid.NewString()
	extclient.TenantID = "tenant-1"

	_extclient, err := ConvertModelsExtClientToSchemaExtclient(extclient)
	require.NoError(t, err)
	assert.Equal(t, network.ID, _extclient.NetworkID)
	assert.Equal(t, "client-convert", _extclient.Name)

	t.Run("ResolvesNetworkByID", func(t *testing.T) {
		_extclient, err := ConvertModelsExtClientToSchemaExtclient(extclient)
		require.NoError(t, err)
		_extclient.Network = nil

		converted := ConvertSchemaExtclientToModelsExtClient(_extclient, SkipViolations())
		assert.Equal(t, network.Name, converted.Network)
	})

	t.Run("RoundTrip", func(t *testing.T) {
		converted := ConvertSchemaExtclientToModelsExtClient(_extclient, SkipViolations())
		converted.LastModified, converted.Mutex = 0, nil
		assert.Equal(t, *extclient, *converted)
	})

	t.Run("NetworkOfAnotherTenant", func(t *testing.T) {
		otherTenant := *extclient
		otherTenant.TenantID = "tenant-2"
		_, err := ConvertModelsExtClientToSchemaExtclient(&otherTenant)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})
}

func createExtClientTestNetwork(t *testing.T, ctx context.Context, name string) *schema.Network {
	t.Helper()

	network := &schema.Network{
		TenantID:     scope.ID(ctx),
		Name:         name,
		AddressRange: "10.0.0.0/24",
	}
	require.NoError(t, network.Create(ctx))
	return network
}

func TestRenameExtClientInAclPolicies(t *testing.T) {
	ctx := extClientTestCtx("tenant-1")
	network := createExtClientTestNetwork(t, ctx, "extclient-acl-net")
	other := createExtClientTestNetwork(t, ctx, "extclient-acl-net-2")

	policy := models.Acl{
		ID:        uuid.NewString(),
		NetworkID: schema.NetworkID(network.Name),
		RuleType:  models.DevicePolicy,
		Src: []models.AclPolicyTag{
			{ID: models.NodeID, Value: "client-old"},
			{ID: models.EgressID, Value: "client-old"},
		},
		Dst: []models.AclPolicyTag{
			{ID: models.NodeID, Value: "client-other"},
			{ID: models.NodeID, Value: "client-old"},
		},
	}
	require.NoError(t, UpsertAcl(ctx, policy))

	otherNetworkPolicy := models.Acl{
		ID:        uuid.NewString(),
		NetworkID: schema.NetworkID(other.Name),
		RuleType:  models.DevicePolicy,
		Src:       []models.AclPolicyTag{{ID: models.NodeID, Value: "client-old"}},
		Dst:       []models.AclPolicyTag{{ID: models.NodeID, Value: "client-other"}},
	}
	require.NoError(t, UpsertAcl(ctx, otherNetworkPolicy))

	require.NoError(t, RenameExtClientInAclPolicies(ctx, network.Name, "client-old", "client-new"))

	got, err := GetAcl(ctx, policy.ID)
	require.NoError(t, err)
	assert.Equal(t, []models.AclPolicyTag{
		{ID: models.NodeID, Value: "client-new"},
		{ID: models.EgressID, Value: "client-old"},
	}, got.Src, "only node references are renamed")
	assert.Equal(t, []models.AclPolicyTag{
		{ID: models.NodeID, Value: "client-other"},
		{ID: models.NodeID, Value: "client-new"},
	}, got.Dst)

	got, err = GetAcl(ctx, otherNetworkPolicy.ID)
	require.NoError(t, err)
	assert.Equal(t, "client-old", got.Src[0].Value, "policies of other networks are untouched")
}
