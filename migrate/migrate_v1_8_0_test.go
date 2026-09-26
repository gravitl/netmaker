package migrate

import (
	"context"
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/orchestrator"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	testutils "github.com/gravitl/netmaker/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
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
	for _, address := range extClientAddresses {
		extClient := models.ExtClient{
			ClientID: uuid.NewString(),
			Network:  network.Name,
			Address:  address,
		}
		key, err := logic.GetRecordKey(extClient.ClientID, extClient.Network)
		require.NoError(t, err)
		record := &schema.ExtClientRecord{Key: key, Value: datatypes.NewJSONType(extClient)}
		require.NoError(t, record.Upsert(ctx))
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
	assert.Equal(t, "10.50.0.2", pool.NodeCursor)
	assert.Equal(t, "10.50.0.254", pool.ExtCursor)

	allocations, err := (&schema.IPAllocation{TenantID: network.TenantID, NetworkID: network.ID}).ListByNetwork(ctx)
	require.NoError(t, err)
	assert.Len(t, allocations, 4)
	for _, allocation := range allocations {
		assert.Equal(t, schema.IPAttached, allocation.State)
		assert.NotEmpty(t, allocation.OwnerID, "address %s has no owner", allocation.Address())
		if allocation.Address().String() == "10.50.0.254" {
			assert.Equal(t, schema.IPOwnerExtClient, allocation.OwnerType)
		} else {
			assert.Equal(t, schema.IPOwnerNode, allocation.OwnerType)
		}
	}

	// allocation continues after the backfilled addresses, skipping the ones in
	// use beyond the cursors.
	orch := orchestrator.GetRepository().NetworkOrchestrator()
	ip, err := orch.AllocateNodeIP(ctx, network, uuid.NewString())
	require.NoError(t, err)
	assert.Equal(t, "10.50.0.3", ip.String())
	ip, err = orch.AllocateNodeIP(ctx, network, uuid.NewString())
	require.NoError(t, err)
	assert.Equal(t, "10.50.0.5", ip.String())
	ip, err = orch.AllocateExtclientIP(ctx, network, uuid.NewString())
	require.NoError(t, err)
	assert.Equal(t, "10.50.0.253", ip.String())
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

func TestInitialIPPoolCursors(t *testing.T) {
	first, last := netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.6")
	allocated := func(addrs ...string) map[netip.Addr]struct{} {
		m := make(map[netip.Addr]struct{})
		for _, addr := range addrs {
			m[netip.MustParseAddr(addr)] = struct{}{}
		}
		return m
	}

	tests := []struct {
		name       string
		allocated  map[netip.Addr]struct{}
		nodeCursor string
		extCursor  string
	}{
		{"Empty", allocated(), "", ""},
		{"Nodes", allocated("10.0.0.1", "10.0.0.2", "10.0.0.4"), "10.0.0.2", ""},
		{"ExtClients", allocated("10.0.0.6", "10.0.0.5"), "", "10.0.0.5"},
		{"Full", allocated("10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4", "10.0.0.5", "10.0.0.6"), "10.0.0.6", "10.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeCursor, extCursor := initialIPPoolCursors(tt.allocated, first, last)
			assert.Equal(t, tt.nodeCursor, nodeCursor)
			assert.Equal(t, tt.extCursor, extCursor)
		})
	}
}
