package migrate

import (
	"context"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/migrate/types"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/orchestrator"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	testutils "github.com/gravitl/netmaker/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func seedIPAllocationsNetwork(t *testing.T, ctx context.Context, nodeAddresses, extClientAddresses []string) *schema.Network {
	t.Helper()

	network := &schema.Network{
		TenantID:     scope.ID(ctx),
		Name:         "ipam-net",
		AddressRange: "10.50.0.0/24",
	}
	require.NoError(t, network.Create(ctx))
	host := testutils.CreateHost(t, ctx, "ipam-host")

	for _, address := range nodeAddresses {
		node := &schema.Node{
			ID:        uuid.NewString(),
			TenantID:  network.TenantID,
			HostID:    host.ID.String(),
			NetworkID: network.ID,
			Address:   address,
		}
		require.NoError(t, node.Create(ctx))
	}
	for i, address := range extClientAddresses {
		extClient := &schema.Extclient{
			TenantID:  network.TenantID,
			NetworkID: network.ID,
			Name:      fmt.Sprintf("ipam-client-%d", i),
			Address:   address,
		}
		require.NoError(t, extClient.Create(ctx))
	}
	return network
}

func TestMigrateIPAllocations(t *testing.T) {
	ctx := scope.WithContext(setupMigrationTest(t), scope.TenantScope, "tenant-1")
	network := seedIPAllocationsNetwork(t, ctx,
		[]string{"10.50.0.1/24", "10.50.0.2/24", "10.50.0.4/24"},
		[]string{"10.50.0.254"},
	)

	require.NoError(t, migrateIPAllocations(ctx))

	pool := &schema.IPPool{TenantID: network.TenantID, NetworkID: network.ID, Family: schema.IPv4}
	require.NoError(t, pool.GetForUpdate(ctx))
	assert.Equal(t, "10.50.0.2", pool.Cursor)

	allocations, err := (&schema.IPAllocation{TenantID: network.TenantID, NetworkID: network.ID}).ListByNetwork(ctx)
	require.NoError(t, err)
	assert.Len(t, allocations, 4)
	for _, allocation := range allocations {
		assert.Equal(t, schema.IPAttached, allocation.State)
		assert.NotEmpty(t, allocation.OwnerID, "address %s has no owner", allocation.Address())
		if allocation.Address().String() == "10.50.0.254" {
			assert.Equal(t, schema.IPOwnerExtClient, allocation.OwnerType)
			extClient := &schema.Extclient{ID: allocation.OwnerID}
			assert.NoError(t, extClient.Get(ctx), "extclient address is not owned by the extclient's id")
		} else {
			assert.Equal(t, schema.IPOwnerNode, allocation.OwnerType)
		}
	}

	// allocation continues after the backfilled addresses, skipping the ones in
	// use beyond the cursor.
	orch := orchestrator.GetRepository().NetworkOrchestrator()
	ip, err := orch.AllocateNodeIP(ctx, network, uuid.NewString())
	require.NoError(t, err)
	assert.Equal(t, "10.50.0.3", ip.String())
	ip, err = orch.AllocateNodeIP(ctx, network, uuid.NewString())
	require.NoError(t, err)
	assert.Equal(t, "10.50.0.5", ip.String())
	ip, err = orch.AllocateExtclientIP(ctx, network, uuid.NewString())
	require.NoError(t, err)
	assert.Equal(t, "10.50.0.6", ip.String())
}

func TestMigrateIPAllocations_AbortsOnDuplicates(t *testing.T) {
	ctx := scope.WithContext(setupMigrationTest(t), scope.TenantScope, "tenant-1")
	seedIPAllocationsNetwork(t, ctx, []string{"10.50.0.1/24"}, []string{"10.50.0.1"})

	err := migrateIPAllocations(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "10.50.0.1")
}

func TestRekeyTenant_IPAllocations(t *testing.T) {
	ctx := setupMigrationTest(t)
	network := seedIPAllocationsNetwork(t, scope.WithContext(ctx, scope.TenantScope, "tenant-1"),
		[]string{"10.50.0.1/24"},
		[]string{"10.50.0.254"},
	)
	require.NoError(t, migrateIPAllocations(ctx))

	require.NoError(t, RekeyTenant(ctx, "tenant-1", "tenant-2"))

	for _, tenantID := range []string{"tenant-1", "tenant-2"} {
		allocations, err := (&schema.IPAllocation{TenantID: tenantID, NetworkID: network.ID}).ListByNetwork(ctx)
		require.NoError(t, err)
		pool := &schema.IPPool{TenantID: tenantID, NetworkID: network.ID, Family: schema.IPv4}
		poolErr := pool.GetForUpdate(ctx)
		if tenantID == "tenant-1" {
			assert.Empty(t, allocations)
			assert.Error(t, poolErr)
		} else {
			assert.Len(t, allocations, 2)
			assert.NoError(t, poolErr)
		}
	}

	// allocation continues in the new tenant.
	tenantCtx := scope.WithContext(ctx, scope.TenantScope, "tenant-2")
	network = &schema.Network{ID: network.ID}
	require.NoError(t, network.Get(tenantCtx))
	ip, err := orchestrator.GetRepository().NetworkOrchestrator().AllocateNodeIP(tenantCtx, network, uuid.NewString())
	require.NoError(t, err)
	assert.Equal(t, "10.50.0.2", ip.String())
}

func TestInitialIPPoolCursor(t *testing.T) {
	first, last := netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.6")
	allocated := func(addrs ...string) map[netip.Addr]struct{} {
		m := make(map[netip.Addr]struct{})
		for _, addr := range addrs {
			m[netip.MustParseAddr(addr)] = struct{}{}
		}
		return m
	}

	tests := []struct {
		name      string
		allocated map[netip.Addr]struct{}
		cursor    string
	}{
		{"Empty", allocated(), ""},
		{"Block At Start", allocated("10.0.0.1", "10.0.0.2", "10.0.0.4"), "10.0.0.2"},
		{"Block At End", allocated("10.0.0.6", "10.0.0.5"), ""},
		{"Full", allocated("10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4", "10.0.0.5", "10.0.0.6"), "10.0.0.6"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.cursor, initialIPPoolCursor(tt.allocated, first, last))
		})
	}
}

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

func TestMigrateAcls(t *testing.T) {
	ctx := setupMigrationTest(t)
	require.NoError(t, db.FromContext(ctx).AutoMigrate(&types.AclRecord{}))

	customID := uuid.NewString()
	for _, record := range []types.AclRecord{
		{
			Key:      "tenant-1::net-1.all-nodes",
			TenantID: "tenant-1",
			Value:    datatypes.NewJSONType(schema.Acl{ID: "net-1.all-nodes", Name: "All Nodes", NetworkID: "net-1"}),
		},
		{
			Key:       "tenant-1::" + customID,
			TenantID:  "tenant-1",
			NetworkID: "net-2",
			Value:     datatypes.NewJSONType(schema.Acl{ID: customID, Name: "custom", NetworkID: "net-2"}),
		},
		{
			Key:      "tenant-2::net-1.all-nodes",
			TenantID: "tenant-2",
			Value:    datatypes.NewJSONType(schema.Acl{ID: "net-1.all-nodes", NetworkID: "net-1"}),
		},
	} {
		require.NoError(t, db.FromContext(ctx).Create(&record).Error)
	}

	require.NoError(t, migrateAcls(ctx))

	tenantCtx := scope.WithContext(ctx, scope.TenantScope, "tenant-1")
	acl := &schema.AclRecord{Slug: "net-1.all-nodes"}
	require.NoError(t, acl.Get(tenantCtx))
	_, err := uuid.Parse(acl.ID)
	assert.NoError(t, err)
	assert.Equal(t, "tenant-1", acl.TenantID)
	assert.Equal(t, "net-1", acl.NetworkID, "the network falls back to the acl's")
	assert.Equal(t, "All Nodes", acl.Value.Data().Name)
	assert.Equal(t, schema.NetworkAccess, acl.Value.Data().AccessType, "legacy acls get network access")

	custom := &schema.AclRecord{Slug: customID}
	require.NoError(t, custom.Get(tenantCtx))
	assert.Equal(t, customID, custom.ID, "uuid acl ids are kept")
	assert.Equal(t, "net-2", custom.NetworkID)

	other := &schema.AclRecord{Slug: "net-1.all-nodes"}
	require.NoError(t, other.Get(scope.WithContext(ctx, scope.TenantScope, "tenant-2")))
	assert.NotEqual(t, acl.ID, other.ID)

	var records int64
	require.NoError(t, db.FromContext(ctx).Model(&types.AclRecord{}).Count(&records).Error)
	assert.Equal(t, int64(3), records, "the old table is kept")
}

func TestMigrateTagsAndDNSEntries(t *testing.T) {
	ctx := setupMigrationTest(t)
	require.NoError(t, db.FromContext(ctx).AutoMigrate(&types.TagRecord{}, &types.DNSRecord{}))
	network := createMigrationTestNetwork(t, ctx, "tenant-1", "net-1")
	otherTenantNetwork := createMigrationTestNetwork(t, ctx, "tenant-2", "net-1")

	createdAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for _, record := range []types.TagRecord{
		{
			Key:      "tenant-1::net-1.tag-1",
			TenantID: "tenant-1",
			Value: datatypes.NewJSONType(types.Tag{
				ID: "net-1.tag-1", TagName: "tag-1", Network: "net-1", ColorCode: "#ffffff", CreatedBy: "admin", CreatedAt: createdAt,
			}),
		},
		{
			Key:      "tenant-2::net-1.tag-1",
			TenantID: "tenant-2",
			Value:    datatypes.NewJSONType(types.Tag{ID: "net-1.tag-1", TagName: "tag-1", Network: "net-1"}),
		},
		{
			Key:      "tenant-1::deleted-net.tag-1",
			TenantID: "tenant-1",
			Value:    datatypes.NewJSONType(types.Tag{ID: "deleted-net.tag-1", TagName: "tag-1", Network: "deleted-net"}),
		},
	} {
		require.NoError(t, db.FromContext(ctx).Create(&record).Error)
	}
	for _, record := range []types.DNSRecord{
		{
			Key:      "tenant-1::host-1###net-1",
			TenantID: "tenant-1",
			Value:    datatypes.NewJSONType(types.DNSEntry{Name: "host-1", Network: "net-1", Address: "10.0.0.10", Address6: "fd00::10"}),
		},
		{
			Key:      "tenant-1::host-1###deleted-net",
			TenantID: "tenant-1",
			Value:    datatypes.NewJSONType(types.DNSEntry{Name: "host-1", Network: "deleted-net", Address: "10.0.0.10"}),
		},
	} {
		require.NoError(t, db.FromContext(ctx).Create(&record).Error)
	}

	require.NoError(t, migrateTags(ctx))
	require.NoError(t, migrateDNSEntries(ctx))

	tenantCtx := scope.WithContext(ctx, scope.TenantScope, "tenant-1")
	tag := &schema.Tag{Network: &schema.Network{Name: "net-1"}, Name: "tag-1"}
	require.NoError(t, tag.Get(tenantCtx))
	assert.Equal(t, network.ID, tag.NetworkID)
	assert.Equal(t, "#ffffff", tag.ColorCode)
	assert.Equal(t, "admin", tag.CreatedBy)
	assert.True(t, createdAt.Equal(tag.CreatedAt))

	otherTag := &schema.Tag{NetworkID: otherTenantNetwork.ID, Name: "tag-1"}
	require.NoError(t, otherTag.Get(scope.WithContext(ctx, scope.TenantScope, "tenant-2")))

	count, err := (&schema.Tag{}).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "tags of deleted networks are skipped")

	entry := &schema.DNSEntry{Network: &schema.Network{Name: "net-1"}, Name: "host-1"}
	require.NoError(t, entry.Get(tenantCtx))
	assert.Equal(t, network.ID, entry.NetworkID)
	assert.Equal(t, "10.0.0.10", entry.Address)
	assert.Equal(t, "fd00::10", entry.Address6)

	count, err = (&schema.DNSEntry{}).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "dns entries of deleted networks are skipped")
}

func TestMigrateUserGroups(t *testing.T) {
	ctx := setupMigrationTest(t)

	customID := uuid.NewString()
	for _, group := range []struct{ id, tenantID string }{
		{"tenant-1::net-1-network-admin-grp", "tenant-1"},
		{"tenant-1::global-network-admin-grp", "tenant-1"},
		{customID, "tenant-1"},
	} {
		require.NoError(t, db.FromContext(ctx).
			Exec("INSERT INTO user_groups_v1 (id, tenant_id, name) VALUES (?, ?, ?)", group.id, group.tenantID, group.id).Error)
	}

	require.NoError(t, migrateUserGroups(ctx))

	tenantCtx := scope.WithContext(ctx, scope.TenantScope, "tenant-1")
	for _, slug := range []schema.UserGroupID{"net-1-network-admin-grp", "global-network-admin-grp"} {
		group := &schema.UserGroup{Slug: slug}
		require.NoError(t, group.Get(tenantCtx), slug)
		_, err := uuid.Parse(group.ID)
		assert.NoError(t, err, "default groups get uuid ids")
		assert.NotEqual(t, slug.String(), group.ID)
	}

	custom := &schema.UserGroup{Slug: schema.UserGroupID(customID)}
	require.NoError(t, custom.Get(tenantCtx))
	assert.Equal(t, customID, custom.ID, "custom group ids are kept")

	require.NoError(t, migrateUserGroups(ctx), "migrating again is a no-op")
	count, err := (&schema.UserGroup{}).Count(tenantCtx)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestMigrateUserRoles(t *testing.T) {
	ctx := setupMigrationTest(t)

	for _, role := range []struct{ id, networkID string }{
		{"super-admin", ""},
		{"org-owner", ""},
		{"tenant-1::net-1-network-admin", "net-1"},
		{"tenant-2::net-1-network-admin", "net-1"},
		{"tenant-1::global-network-admin", "all_networks"},
	} {
		require.NoError(t, db.FromContext(ctx).
			Exec("INSERT INTO user_roles_v1 (id, network_id, name) VALUES (?, ?, ?)", role.id, role.networkID, role.id).Error)
	}

	require.NoError(t, migrateUserRoles(ctx))

	for _, slug := range []schema.UserRoleID{schema.SuperAdminRole, schema.OrgOwner} {
		role := &schema.UserRole{Slug: slug}
		require.NoError(t, role.GetPlatformRole(ctx), slug)
		assert.Equal(t, scope.GlobalScope, role.Scope)
		assert.Empty(t, role.ScopeID)
		_, err := uuid.Parse(role.ID)
		assert.NoError(t, err)
	}

	for _, tenantID := range []string{"tenant-1", "tenant-2"} {
		role := &schema.UserRole{Slug: "net-1-network-admin"}
		require.NoError(t, role.GetNetworkRole(scope.WithContext(ctx, scope.TenantScope, tenantID)), tenantID)
		assert.Equal(t, scope.TenantScope, role.Scope)
		assert.Equal(t, tenantID, role.ScopeID)
	}

	allNetworks := &schema.UserRole{Slug: "global-network-admin"}
	require.NoError(t, allNetworks.GetNetworkRole(scope.WithContext(ctx, scope.TenantScope, "tenant-1")))
	assert.Equal(t, schema.AllNetworks, allNetworks.NetworkID)

	roles, err := (&schema.UserRole{}).ListNetworkRoles(scope.WithContext(ctx, scope.TenantScope, "tenant-1"))
	require.NoError(t, err)
	assert.Len(t, roles, 2)
}
