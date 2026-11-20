package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/c-robinson/iplib"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/orchestrator"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func tenantCtx() context.Context {
	return scope.WithContext(db.WithContext(context.Background()), scope.TenantScope, defaultTenantID)
}

func createIngressFixture(t *testing.T, ctx context.Context, netName, cidr string) (*schema.Network, *schema.Host, *schema.Node) {
	t.Helper()
	network := &schema.Network{
		ID:           uuid.NewString(),
		TenantID:     scope.ID(ctx),
		Name:         netName,
		AddressRange: cidr,
	}
	require.NoError(t, network.Create(ctx))

	host := &schema.Host{
		ID:         uuid.New(),
		TenantID:   scope.ID(ctx),
		Name:       netName + "-host",
		EndpointIP: net.ParseIP("203.0.113.10"),
		ListenPort: 51821,
	}
	require.NoError(t, host.Create(ctx))

	node, err := orchestrator.GetRepository().NodeOrchestrator().CreateNode(ctx, host, network)
	require.NoError(t, err)
	node.IsGateway = true
	require.NoError(t, node.Update(ctx))

	t.Cleanup(func() {
		if clients, err := logic.GetNetworkExtClients(ctx, network.Name); err == nil {
			for _, c := range clients {
				_ = logic.DeleteExtClient(ctx, c.Network, c.ClientID, false)
			}
		}
		_ = node.Delete(ctx)
		_ = host.Delete(ctx)
		_ = network.Delete(ctx)
	})
	return network, host, node
}

func TestBulkCreateExtClients(t *testing.T) {
	ctx := tenantCtx()
	network, host, node := createIngressFixture(t, ctx, "bulk-ext-net", "10.201.0.0/22")

	const count = 25
	clients := make([]models.CustomExtClient, count)
	for i := 0; i < count; i++ {
		clients[i] = models.CustomExtClient{
			ClientID: fmt.Sprintf("bulk-%d", i),
		}
	}
	nodeModel, err := logic.GetNodeByID(node.ID)
	require.NoError(t, err)
	resp := createExtClientsInBulk(ctx, network.Name, nodeModel, host, logic.MasterUser, clients)
	assert.Len(t, resp.Created, count)
	assert.Empty(t, resp.Failed)

	seen := make(map[string]struct{}, count)
	ids := make(map[string]struct{}, count)
	for _, c := range resp.Created {
		assert.Equal(t, network.Name, c.Network)
		assert.Equal(t, node.ID, c.IngressGatewayID)
		assert.NotEmpty(t, c.Address)
		assert.NotEmpty(t, c.PublicKey)
		_, dup := seen[c.Address]
		assert.False(t, dup, "duplicate address %s", c.Address)
		seen[c.Address] = struct{}{}
		ids[c.ClientID] = struct{}{}
	}
	for i := 0; i < count; i++ {
		_, ok := ids[fmt.Sprintf("bulk-%d", i)]
		assert.True(t, ok, "caller client id bulk-%d was replaced", i)
	}

	stored, err := logic.GetNetworkExtClients(ctx, network.Name)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(stored), count)

	_ = host // host used via node
}

func TestBulkCreateExtClients_RejectsOverLimit(t *testing.T) {
	ctx := tenantCtx()
	network, _, node := createIngressFixture(t, ctx, "bulk-limit-net", "10.202.0.0/16")

	clients := make([]models.CustomExtClient, maxBulkExtClientCreate+1)
	for i := range clients {
		clients[i] = models.CustomExtClient{ClientID: fmt.Sprintf("over-%d", i)}
	}
	body, err := json.Marshal(models.BulkCreateExtClientRequest{
		IngressGatewayID: node.ID,
		Clients:          clients,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/extclients/"+network.Name+"/bulk", bytes.NewReader(body))
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"network": network.Name})
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("user", "admin")
	req.Header.Set("ismaster", "yes")

	rec := httptest.NewRecorder()
	bulkCreateExtClients(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestBulkCreateExtClients_ReturnsAccepted(t *testing.T) {
	ctx := tenantCtx()
	network, _, node := createIngressFixture(t, ctx, "bulk-accepted-net", "10.208.0.0/24")

	body, err := json.Marshal(models.BulkCreateExtClientRequest{
		IngressGatewayID: node.ID,
		Count:            2,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/extclients/"+network.Name+"/bulk", bytes.NewReader(body))
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"network": network.Name})
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("user", "admin")
	req.Header.Set("ismaster", "yes")

	rec := httptest.NewRecorder()
	bulkCreateExtClients(rec, req)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		stored, err := logic.GetNetworkExtClients(ctx, network.Name)
		require.NoError(t, err)
		if len(stored) >= 2 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("bulk create did not finish in the background")
}

func TestBulkCreateExtClientsByCount(t *testing.T) {
	ctx := tenantCtx()
	network, host, node := createIngressFixture(t, ctx, "bulk-count-net", "10.206.0.0/24")

	nodeModel, err := logic.GetNodeByID(node.ID)
	require.NoError(t, err)
	nodeIP := nodeModel.Address.IP

	resp := createExtClientsInBulk(ctx, network.Name, nodeModel, host, logic.MasterUser, make([]models.CustomExtClient, 3))
	assert.Len(t, resp.Created, 3)
	assert.Empty(t, resp.Failed)
	idRe := regexp.MustCompile(`^ec-[0-9a-f]{8}$`)
	seenID := map[string]struct{}{}
	seenAddr := map[string]struct{}{}
	for _, c := range resp.Created {
		assert.Regexp(t, idRe, c.ClientID)
		assert.NotEmpty(t, c.Address)
		assert.NotEqual(t, nodeIP.String(), c.Address)
		_, dupID := seenID[c.ClientID]
		assert.False(t, dupID)
		seenID[c.ClientID] = struct{}{}
		_, dupAddr := seenAddr[c.Address]
		assert.False(t, dupAddr)
		seenAddr[c.Address] = struct{}{}
	}

	again, err := orchestrator.GetRepository().NetworkOrchestrator().AllocateExtclientIP(ctx, network)
	require.NoError(t, err)
	_, reused := seenAddr[again.String()]
	assert.False(t, reused, "second allocation reused %s", again.String())
	orchestrator.GetRepository().NetworkOrchestrator().FreeIPv4Reservation(network.ID, again.String())
}

func TestBulkCreateExtClients_AssignsTags(t *testing.T) {
	ctx := tenantCtx()
	network, host, node := createIngressFixture(t, ctx, "bulk-tag-net", "10.209.0.0/24")
	tagID := models.TagID(network.Name + ".sales")
	require.NoError(t, (&schema.TagRecord{
		Key: tagID.String(),
		Value: datatypes.NewJSONType(schema.Tag{
			ID:      tagID,
			TagName: "sales",
			Network: schema.NetworkID(network.Name),
		}),
	}).Upsert(ctx))
	t.Cleanup(func() {
		_ = (&schema.TagRecord{Key: tagID.String()}).Delete(ctx)
	})

	nodeModel, err := logic.GetNodeByID(node.ID)
	require.NoError(t, err)
	var tagged models.CustomExtClient
	require.NoError(t, json.Unmarshal([]byte(`{"clientid":"tagged-0","tags":["sales"]}`), &tagged))
	resp := createExtClientsInBulk(ctx, network.Name, nodeModel, host, logic.MasterUser, []models.CustomExtClient{tagged})
	require.Empty(t, resp.Failed)
	require.Len(t, resp.Created, 1)
	_, ok := resp.Created[0].Tags[tagID]
	assert.True(t, ok)

	countClients := make([]models.CustomExtClient, 1)
	applyBulkCreateTags(countClients, []string{"sales"})
	resp = createExtClientsInBulk(ctx, network.Name, nodeModel, host, logic.MasterUser, countClients)
	require.Empty(t, resp.Failed)
	require.Len(t, resp.Created, 1)
	_, ok = resp.Created[0].Tags[tagID]
	assert.True(t, ok)

	body, err := json.Marshal(models.BulkCreateExtClientRequest{
		IngressGatewayID: node.ID,
		Count:            1,
		Tags:             []string{"missing-tag"},
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/extclients/"+network.Name+"/bulk", bytes.NewReader(body))
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"network": network.Name})
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("user", "admin")
	req.Header.Set("ismaster", "yes")
	rec := httptest.NewRecorder()
	bulkCreateExtClients(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestBulkCreateExtClients_RejectsCountAndClients(t *testing.T) {
	ctx := tenantCtx()
	network, _, node := createIngressFixture(t, ctx, "bulk-both-net", "10.207.0.0/24")

	body, err := json.Marshal(models.BulkCreateExtClientRequest{
		IngressGatewayID: node.ID,
		Count:            2,
		Clients:          []models.CustomExtClient{{ClientID: "named-0"}},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/extclients/"+network.Name+"/bulk", bytes.NewReader(body))
	req = req.WithContext(ctx)
	req = mux.SetURLVars(req, map[string]string{"network": network.Name})
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("user", "admin")
	req.Header.Set("ismaster", "yes")

	rec := httptest.NewRecorder()
	bulkCreateExtClients(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetStaticNodesByNetwork_IsNetworkScoped(t *testing.T) {
	ctx := tenantCtx()
	netA, _, _ := createIngressFixture(t, ctx, "static-scope-a", "10.203.0.0/24")
	netB, _, _ := createIngressFixture(t, ctx, "static-scope-b", "10.204.0.0/24")

	for i := 0; i < 3; i++ {
		require.NoError(t, logic.SaveExtClient(ctx, &models.ExtClient{
			ClientID:         fmt.Sprintf("a-%d", i),
			Network:          netA.Name,
			Address:          fmt.Sprintf("10.203.0.%d", i+10),
			Enabled:          true,
			IngressGatewayID: "gw-a",
			PublicKey:        "key-a",
		}))
		require.NoError(t, logic.SaveExtClient(ctx, &models.ExtClient{
			ClientID:             fmt.Sprintf("b-%d", i),
			Network:              netB.Name,
			Address:              fmt.Sprintf("10.204.0.%d", i+10),
			Enabled:              true,
			IngressGatewayID:     "gw-b",
			PublicKey:            "key-b",
			RemoteAccessClientID: "",
		}))
	}
	// RAC client on netB should be excluded when onlyWg=true
	require.NoError(t, logic.SaveExtClient(ctx, &models.ExtClient{
		ClientID:             "b-rac",
		Network:              netB.Name,
		Address:              "10.204.0.50",
		Enabled:              true,
		IngressGatewayID:     "gw-b",
		PublicKey:            "key-rac",
		RemoteAccessClientID: "rac-device",
	}))

	staticA := logic.GetStaticNodesByNetwork(ctx, schema.NetworkID(netA.Name), false)
	assert.Len(t, staticA, 3)
	for _, n := range staticA {
		assert.Equal(t, netA.Name, n.Network)
	}

	staticBAll := logic.GetStaticNodesByNetwork(ctx, schema.NetworkID(netB.Name), false)
	assert.Len(t, staticBAll, 4)

	staticBWg := logic.GetStaticNodesByNetwork(ctx, schema.NetworkID(netB.Name), true)
	assert.Len(t, staticBWg, 3)
}

func TestAllocateExtclientIP_DenseViaHandlerPath(t *testing.T) {
	ctx := tenantCtx()
	network, _, node := createIngressFixture(t, ctx, "dense-alloc-net", "10.205.0.0/24")

	net4 := iplib.Net4FromStr(network.AddressRange)
	addr := net4.LastAddress()
	const seedCount = 150
	for i := 0; i < seedCount; i++ {
		require.NoError(t, logic.SaveExtClient(ctx, &models.ExtClient{
			ClientID:         fmt.Sprintf("dense-%d", i),
			Network:          network.Name,
			Address:          addr.String(),
			Enabled:          true,
			IngressGatewayID: node.ID,
			PublicKey:        "dense-key",
		}))
		prev, err := net4.PreviousIP(addr)
		require.NoError(t, err)
		addr = prev
	}

	orch := orchestrator.GetRepository().NetworkOrchestrator()
	start := time.Now()
	ip, err := orch.AllocateExtclientIP(ctx, network)
	elapsed := time.Since(start)
	require.NoError(t, err)
	require.NotNil(t, ip)
	assert.Less(t, elapsed, 2*time.Second, "allocation took %s", elapsed)
	orch.FreeIPv4Reservation(network.ID, ip.String())
}
