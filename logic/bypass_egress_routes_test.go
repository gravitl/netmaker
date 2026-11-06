package logic

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestInternetEgressBypassesEgressRoutes(t *testing.T) {
	assert.False(t, InternetEgressBypassesEgressRoutes(schema.Egress{
		Type:               schema.EgressTypeCIDR,
		Range:              "10.0.0.0/8",
		BypassEgressRoutes: true,
	}))
	assert.False(t, InternetEgressBypassesEgressRoutes(schema.Egress{
		Type:               schema.EgressTypeInternet,
		Range:              "*",
		BypassEgressRoutes: false,
	}))
	assert.True(t, InternetEgressBypassesEgressRoutes(schema.Egress{
		Type:               schema.EgressTypeInternet,
		Range:              "*",
		BypassEgressRoutes: true,
	}))
	assert.True(t, InternetEgressBypassesEgressRoutes(schema.Egress{
		Type:               schema.EgressTypeCIDR,
		Range:              "*",
		BypassEgressRoutes: true,
	}))
}

func TestResolveBypassEgressRoutesForCreate(t *testing.T) {
	assert.False(t, ResolveBypassEgressRoutesForCreate(nil, false))
	assert.True(t, ResolveBypassEgressRoutesForCreate(nil, true))

	f := false
	tr := true
	assert.False(t, ResolveBypassEgressRoutesForCreate(&models.EgressReq{BypassEgressRoutes: &f}, true))
	assert.True(t, ResolveBypassEgressRoutesForCreate(&models.EgressReq{BypassEgressRoutes: &tr}, true))
	assert.False(t, ResolveBypassEgressRoutesForCreate(&models.EgressReq{BypassEgressRoutes: &tr}, false))
}

func TestResolveBypassEgressRoutesForUpdate(t *testing.T) {
	assert.False(t, ResolveBypassEgressRoutesForUpdate(nil, false, true))
	assert.True(t, ResolveBypassEgressRoutesForUpdate(nil, true, true), "omit preserves previous true")
	assert.False(t, ResolveBypassEgressRoutesForUpdate(nil, true, false), "omit preserves previous false")

	f := false
	tr := true
	assert.False(t, ResolveBypassEgressRoutesForUpdate(&models.EgressReq{BypassEgressRoutes: &f}, true, true))
	assert.True(t, ResolveBypassEgressRoutesForUpdate(&models.EgressReq{BypassEgressRoutes: &tr}, true, false))
}

func TestPeerAdvertisesSpecificEgress(t *testing.T) {
	assert.False(t, PeerAdvertisesSpecificEgress(nil))
	assert.False(t, PeerAdvertisesSpecificEgress(&models.Node{}))

	onlyDefaults := &models.Node{}
	onlyDefaults.EgressDetails = models.EgressDetails{
		IsEgressGateway:     true,
		EgressGatewayRanges: []string{IPv4Network, IPv6Network},
	}
	assert.False(t, PeerAdvertisesSpecificEgress(onlyDefaults))

	withSite := &models.Node{}
	withSite.EgressDetails = models.EgressDetails{
		IsEgressGateway:     true,
		EgressGatewayRanges: []string{IPv4Network, "10.20.0.0/16", "10.30.0.0/16"},
	}
	assert.True(t, PeerAdvertisesSpecificEgress(withSite))

	nested := &models.Node{}
	nested.EgressDetails = models.EgressDetails{
		IsEgressGateway: true,
		EgressGatewayRequest: models.EgressGatewayRequest{
			RangesWithMetric: []models.EgressRangeMetric{
				{Network: "10.0.0.0/8"},
				{Network: "10.10.0.0/16"},
			},
		},
	}
	assert.True(t, PeerAdvertisesSpecificEgress(nested))
}

func TestShouldRetainPeerDespiteRelay_ExitPeerAlways(t *testing.T) {
	exitID := uuid.New()
	clientID := uuid.New()
	client := &models.Node{
		CommonNode: models.CommonNode{
			ID:      clientID,
			Network: "testnet",
		},
	}
	client.InternetGwID = exitID.String()
	client.IsRelayed = true
	client.RelayedBy = exitID.String()

	exit := &models.Node{
		CommonNode: models.CommonNode{
			ID:      exitID,
			Network: "testnet",
		},
	}
	assert.True(t, shouldRetainPeerDespiteRelay(client, exit, false, false, nil))
}

func TestShouldRetainPeerDespiteRelay_NoBypassWithoutSelection(t *testing.T) {
	clientID := uuid.New()
	siteID := uuid.New()
	client := &models.Node{
		CommonNode: models.CommonNode{
			ID:      clientID,
			Network: "testnet",
		},
	}
	client.IsRelayed = true
	client.RelayedBy = uuid.New().String()
	// No SelectedInternetEgressID → bypass lookup fails open to false.

	site := &models.Node{
		CommonNode: models.CommonNode{
			ID:      siteID,
			Network: "testnet",
		},
	}
	site.EgressDetails = models.EgressDetails{
		IsEgressGateway:     true,
		EgressGatewayRanges: []string{"10.20.0.0/16"},
	}
	assert.False(t, shouldRetainPeerDespiteRelay(client, site, false, false, nil),
		"without selected internet egress, specific egress peers must not be retained via bypass")
}

func TestShouldRetainPeerDespiteRelay_NonIGWRelayedStillRemoves(t *testing.T) {
	// Relayed (non-exit) client with no internet selection must not retain site peers
	// via the bypass helper — GetPeerUpdateForHost still removes them.
	client := &models.Node{
		CommonNode: models.CommonNode{
			ID:      uuid.New(),
			Network: "testnet",
		},
	}
	client.IsRelayed = true
	client.RelayedBy = uuid.New().String()

	meshPeer := &models.Node{
		CommonNode: models.CommonNode{
			ID:      uuid.New(),
			Network: "testnet",
		},
	}
	require.False(t, PeerAdvertisesSpecificEgress(meshPeer))
	assert.False(t, shouldRetainPeerDespiteRelay(client, meshPeer, false, false, nil))
}

func TestShouldRetainPeerDespiteRelay_SpecificEgressKeepsBypassClient(t *testing.T) {
	// Reverse path: when peer.IsRelayed, GetPeerUpdateForHost removes peers unless
	// shouldRetainPeerDespiteRelay(site, client) is true. That requires
	// SelectedInternetEgressBypasses(client) && PeerAdvertisesSpecificEgress(site).
	// Without a resolvable selected internet egress, bypass is false → do not retain.
	exitID := uuid.New()
	client := &models.Node{
		CommonNode: models.CommonNode{
			ID:      uuid.New(),
			Network: "testnet",
		},
	}
	client.IsRelayed = true
	client.RelayedBy = exitID.String()
	client.InternetGwID = exitID.String()
	client.SelectedInternetEgressID = "inet-eg-missing"

	site := &models.Node{
		CommonNode: models.CommonNode{
			ID:      uuid.New(),
			Network: "testnet",
		},
	}
	site.EgressDetails = models.EgressDetails{
		IsEgressGateway:     true,
		EgressGatewayRanges: []string{"10.20.0.0/16"},
	}
	require.True(t, PeerAdvertisesSpecificEgress(site))
	assert.False(t, shouldRetainPeerDespiteRelay(site, client, false, false, nil),
		"without resolvable BypassEgressRoutes on client, GW must not retain")
}

func TestCanBypassForceDirectPeer_RelayedSiteBlocksReverseRetain(t *testing.T) {
	// Relayed site egress must not keep the bypass exit client as a direct peer;
	// canBypassForceDirectPeer(client, site) is false so reverse retain is off.
	clientID := uuid.New()
	client := &models.Node{CommonNode: models.CommonNode{ID: clientID, Network: "testnet"}}
	client.IsRelayed = true
	client.RelayedBy = uuid.New().String()
	client.InternetGwID = client.RelayedBy

	site := &models.Node{CommonNode: models.CommonNode{ID: uuid.New(), Network: "testnet"}}
	site.IsRelayed = true
	site.RelayedBy = uuid.New().String()
	site.EgressDetails = models.EgressDetails{
		IsEgressGateway:     true,
		EgressGatewayRanges: []string{"10.20.0.0/16"},
	}
	assert.False(t, canBypassForceDirectPeer(client, site, false),
		"client must not force-direct a relayed site")
	assert.False(t, siteAutoRelayedByClient(client, site))
	// Reverse retain uses the same reachability gate.
	assert.False(t, canBypassForceDirectPeer(client, site, siteAutoRelayedByClient(client, site)))

	// GetAllowedIpsForRelayed must only omit the bypass client under RelayedBy when
	// that client is kept as a direct peer of the site (canBypassForceDirectPeer).
	// Relayed sites cannot keep the client direct — the client's /32 must ride
	// RelayedBy so return traffic (e.g. ICMP reply) has a path.
	skipUnderRelayedBy := PeerAdvertisesSpecificEgress(site) &&
		canBypassForceDirectPeer(client, site, siteAutoRelayedByClient(client, site))
	assert.False(t, skipUnderRelayedBy,
		"relayed site egress must advertise bypass client under RelayedBy")
}

func TestFilterEgressDetailsByEgressIDsKeepsOnlyAllowed(t *testing.T) {
	details := models.EgressDetails{
		IsEgressGateway:     true,
		EgressGatewayRanges: []string{"10.1.0.0/16", "10.2.0.0/16", "0.0.0.0/0"},
		EgressGatewayRequest: models.EgressGatewayRequest{
			RangesWithMetric: []models.EgressRangeMetric{
				{EgressID: "allowed", Network: "10.1.0.0/16"},
				{EgressID: "denied", Network: "10.2.0.0/16"},
				{EgressID: "allowed", Network: "0.0.0.0/0"},
			},
		},
	}
	got := filterEgressDetailsByEgressIDs(details, map[string]struct{}{"allowed": {}})
	require.True(t, got.IsEgressGateway)
	assert.Equal(t, []string{"10.1.0.0/16"}, got.EgressGatewayRanges)
	require.Len(t, got.EgressGatewayRequest.RangesWithMetric, 1)
	assert.Equal(t, "allowed", got.EgressGatewayRequest.RangesWithMetric[0].EgressID)

	assert.False(t, filterEgressDetailsByEgressIDs(details, nil).IsEgressGateway)
}

func TestAuthorizedEgressDetailsDropsUnauthorizedRanges(t *testing.T) {
	nodeID := uuid.New()
	peerID := uuid.New()
	node := &models.Node{CommonNode: models.CommonNode{ID: nodeID, Network: "testnet"}}
	peer := &models.Node{CommonNode: models.CommonNode{ID: peerID, Network: "testnet"}}
	eli := []schema.Egress{
		{ID: "allowed", Network: "testnet", Status: true, Range: "10.1.0.0/16", Nodes: datatypes.JSONMap{peerID.String(): float64(256)}},
		{ID: "denied", Network: "testnet", Status: true, Range: "10.2.0.0/16", Nodes: datatypes.JSONMap{peerID.String(): float64(256)}},
	}
	details := models.EgressDetails{
		IsEgressGateway:     true,
		EgressGatewayRanges: []string{"10.1.0.0/16", "10.2.0.0/16"},
		EgressGatewayRequest: models.EgressGatewayRequest{
			RangesWithMetric: []models.EgressRangeMetric{
				{EgressID: "allowed", Network: "10.1.0.0/16"},
				{EgressID: "denied", Network: "10.2.0.0/16"},
			},
		},
	}
	got := authorizedEgressDetails(context.Background(), node, peer, eli, nil, details)
	assert.False(t, got.IsEgressGateway, "ranges without egress access must not be restored")
	assert.Empty(t, got.EgressGatewayRanges)
}

func TestPeerRidesRetainedInternetExit(t *testing.T) {
	originalGetNodeByID := getNodeByID
	originalIsPeerAllowed := IsPeerAllowed
	originalUserAllowed := IsUserAllowedToCommunicate
	t.Cleanup(func() {
		getNodeByID = originalGetNodeByID
		IsPeerAllowed = originalIsPeerAllowed
		IsUserAllowedToCommunicate = originalUserAllowed
	})

	exitA := uuid.New()
	exitB := uuid.New()
	userClient := &models.Node{
		CommonNode: models.CommonNode{ID: uuid.New(), Network: "testnet"},
		OwnerID:    "abhi",
	}
	userClient.InternetGwID = exitA.String()
	userClient.RelayedBy = exitA.String()
	userClient.IsRelayed = true
	require.True(t, IsUserOwnedDevice(userClient))

	serverClient := &models.Node{
		CommonNode: models.CommonNode{ID: uuid.New(), Network: "testnet"},
	}
	serverClient.InternetGwID = exitA.String()
	serverClient.RelayedBy = exitA.String()
	serverClient.IsRelayed = true
	require.False(t, IsUserOwnedDevice(serverClient))

	siteOnB := &models.Node{
		CommonNode: models.CommonNode{ID: uuid.New(), Network: "testnet"},
	}
	siteOnB.IsRelayed = true
	siteOnB.RelayedBy = exitB.String()

	exitBNode := models.Node{CommonNode: models.CommonNode{ID: exitB, Network: "testnet"}}
	getNodeByID = func(id string) (models.Node, error) {
		if id == exitB.String() {
			return exitBNode, nil
		}
		return models.Node{}, assert.AnError
	}
	IsPeerAllowed = func(ctx context.Context, node, peer models.Node, checkDefaultPolicy bool) bool {
		return peer.ID == exitB
	}
	// User devices use user policies via PeerAllowed, not resource IsPeerAllowed.
	IsUserAllowedToCommunicate = func(_ context.Context, userName string, peer models.Node) (bool, []models.Acl) {
		return userName == "abhi" && peer.ID == exitB, nil
	}

	inetExits := map[string]struct{}{exitA.String(): {}, exitB.String(): {}}
	assert.True(t, peerRidesRetainedInternetExit(context.Background(), userClient, siteOnB, exitA.String(), inetExits, false),
		"user device: site RelayedBy another ACL-allowed exit must ride that exit, not the client's")
	assert.False(t, peerRidesRetainedInternetExit(context.Background(), serverClient, siteOnB, exitA.String(), inetExits, false),
		"server exit client does not retain alternate exits — hairpin via selected exit")

	siteOnA := &models.Node{
		CommonNode: models.CommonNode{ID: uuid.New(), Network: "testnet"},
	}
	siteOnA.IsRelayed = true
	siteOnA.RelayedBy = exitA.String()
	assert.False(t, peerRidesRetainedInternetExit(context.Background(), userClient, siteOnA, exitA.String(), inetExits, false),
		"site RelayedBy the client's own exit still rides GetAllowedIpsForRelayed on that exit")

	IsUserAllowedToCommunicate = func(context.Context, string, models.Node) (bool, []models.Acl) {
		return false, nil
	}
	assert.False(t, peerRidesRetainedInternetExit(context.Background(), userClient, siteOnB, exitA.String(), inetExits, false),
		"ACL-denied RelayedBy exit is not retained — keep hairpin via client's exit")
}

func TestCanBypassForceDirectPeer(t *testing.T) {
	clientID := uuid.New()
	client := &models.Node{CommonNode: models.CommonNode{ID: clientID, Network: "testnet"}}

	directSite := &models.Node{
		CommonNode: models.CommonNode{ID: uuid.New(), Network: "testnet"},
	}
	assert.True(t, canBypassForceDirectPeer(client, directSite, false),
		"reachable specific-egress peer may stay direct under bypass")

	assert.False(t, canBypassForceDirectPeer(client, directSite, true),
		"auto-relayed egress peer must not be forced direct; AllowedIPs go via exit")

	relayedSite := &models.Node{
		CommonNode: models.CommonNode{ID: uuid.New(), Network: "testnet"},
	}
	relayedSite.IsRelayed = true
	relayedSite.RelayedBy = uuid.New().String()
	assert.False(t, canBypassForceDirectPeer(client, relayedSite, false),
		"manually relayed egress peer must not be forced direct; AllowedIPs go via exit")

	selfRelayed := &models.Node{
		CommonNode: models.CommonNode{ID: uuid.New(), Network: "testnet"},
	}
	selfRelayed.IsRelayed = true
	selfRelayed.RelayedBy = clientID.String()
	assert.True(t, canBypassForceDirectPeer(client, selfRelayed, false),
		"egress relayed by the client itself may stay direct")

	assert.False(t, canBypassForceDirectPeer(nil, directSite, false))
	assert.False(t, canBypassForceDirectPeer(client, nil, false))
}

func TestShouldRetainPeerDespiteRelay_UnfilteredSpecificNeedsBypass(t *testing.T) {
	exitID := uuid.New()
	client := &models.Node{
		CommonNode: models.CommonNode{
			ID:      uuid.New(),
			Network: "testnet",
		},
	}
	client.IsRelayed = true
	client.RelayedBy = exitID.String()
	client.InternetGwID = exitID.String()

	site := &models.Node{
		CommonNode: models.CommonNode{
			ID:      uuid.New(),
			Network: "testnet",
		},
	}
	require.False(t, PeerAdvertisesSpecificEgress(site))
	assert.False(t, shouldRetainPeerDespiteRelay(client, site, false, false, nil),
		"specific egress cleared by access filtering must not be retained")
}

func TestShouldRetainPeerDespiteRelay_AlternateExitsNeedACL(t *testing.T) {
	selectedExitID := uuid.New()
	otherExitID := uuid.New()
	userClient := &models.Node{
		CommonNode: models.CommonNode{
			ID:      uuid.New(),
			Network: "testnet",
		},
		OwnerID: "abhi",
	}
	userClient.IsRelayed = true
	userClient.RelayedBy = selectedExitID.String()
	userClient.InternetGwID = selectedExitID.String()
	require.True(t, IsUserOwnedDevice(userClient))

	serverClient := &models.Node{
		CommonNode: models.CommonNode{
			ID:      uuid.New(),
			Network: "testnet",
		},
	}
	serverClient.IsRelayed = true
	serverClient.RelayedBy = selectedExitID.String()
	serverClient.InternetGwID = selectedExitID.String()
	require.False(t, IsUserOwnedDevice(serverClient))

	selectedExit := &models.Node{
		CommonNode: models.CommonNode{
			ID:      selectedExitID,
			Network: "testnet",
		},
	}
	otherExit := &models.Node{
		CommonNode: models.CommonNode{
			ID:      otherExitID,
			Network: "testnet",
		},
	}
	exitIDs := map[string]struct{}{
		selectedExitID.String(): {},
		otherExitID.String():    {},
	}
	assert.True(t, shouldRetainPeerDespiteRelay(userClient, selectedExit, false, false, exitIDs),
		"selected exit must stay as a direct peer even without PeerAllowed")
	assert.True(t, shouldRetainPeerDespiteRelay(selectedExit, userClient, false, false, exitIDs),
		"selected exit must keep its client as a direct peer")
	assert.False(t, shouldRetainPeerDespiteRelay(userClient, otherExit, false, false, exitIDs),
		"ACL-denied alternate exits must not be retained")
	assert.False(t, shouldRetainPeerDespiteRelay(otherExit, userClient, false, false, exitIDs),
		"exit must not keep unrelated clients without ACL")
	assert.True(t, shouldRetainPeerDespiteRelay(userClient, otherExit, false, true, exitIDs),
		"ACL-allowed alternate exits stay as separate direct peers for user-device exit clients")
	assert.True(t, shouldRetainPeerDespiteRelay(otherExit, userClient, false, true, exitIDs),
		"ACL-allowed exit keeps user-device exit clients as direct peers")

	assert.True(t, shouldRetainPeerDespiteRelay(serverClient, selectedExit, false, false, exitIDs),
		"server exit client keeps its selected exit")
	assert.True(t, shouldRetainPeerDespiteRelay(selectedExit, serverClient, false, false, exitIDs),
		"selected exit keeps its server client")
	assert.False(t, shouldRetainPeerDespiteRelay(serverClient, otherExit, false, true, exitIDs),
		"server exit clients must not keep alternate exits as separate peers")
	assert.False(t, shouldRetainPeerDespiteRelay(otherExit, serverClient, false, true, exitIDs),
		"alternate exits must not keep server exit clients as direct peers")
}

func TestShouldRetainPeerDespiteRelay_RelayedNonExitDoesNotKeepExitDirect(t *testing.T) {
	relayID := uuid.New()
	exitID := uuid.New()
	// Manually relayed mesh node — not using an internet exit.
	node := &models.Node{
		CommonNode: models.CommonNode{
			ID:      uuid.New(),
			Network: "testnet",
		},
	}
	node.IsRelayed = true
	node.RelayedBy = relayID.String()

	exit := &models.Node{
		CommonNode: models.CommonNode{
			ID:      exitID,
			Network: "testnet",
		},
	}
	exitIDs := map[string]struct{}{exitID.String(): {}}
	assert.False(t, shouldRetainPeerDespiteRelay(node, exit, false, true, exitIDs),
		"relayed non-exit clients must not keep internet exits as separate peers")
	assert.False(t, shouldRetainPeerDespiteRelay(exit, node, false, true, exitIDs),
		"exit must not retain unrelated relayed non-exit clients as direct peers")
}

func TestShouldRetainPeerDespiteRelay_RelayedUserDeviceDropsOtherGateways(t *testing.T) {
	relayID := uuid.New()
	gwID := uuid.New()
	// User device relayed from the UI must not keep other gateways as direct
	// peers. Their overlays ride RelayedBy; keeping both duplicates AllowedIPs
	// and leaves those peers with an empty list.
	userDev := &models.Node{
		CommonNode: models.CommonNode{
			ID:      uuid.New(),
			Network: "testnet",
		},
		OwnerID: "abhi",
	}
	userDev.IsRelayed = true
	userDev.RelayedBy = relayID.String()
	require.True(t, IsUserOwnedDevice(userDev))
	require.False(t, nodeIsInternetExitClient(userDev))

	gw := &models.Node{
		CommonNode: models.CommonNode{
			ID:      gwID,
			Network: "testnet",
		},
	}
	assert.False(t, shouldRetainPeerDespiteRelay(userDev, gw, false, true, nil),
		"relayed user device must not keep other gateways as direct peers")

	relayedDest := &models.Node{
		CommonNode: models.CommonNode{
			ID:      uuid.New(),
			Network: "testnet",
		},
	}
	relayedDest.IsRelayed = true
	relayedDest.RelayedBy = gwID.String()
	assert.False(t, shouldRetainPeerDespiteRelay(userDev, relayedDest, false, true, nil),
		"user device must not force-direct a relayed destination")
}

func TestShouldRetainPeerDespiteRelay_UserDeviceOnExitDoesNotKeepPlainGateway(t *testing.T) {
	exitID := uuid.New()
	gwID := uuid.New()
	userDev := &models.Node{
		CommonNode: models.CommonNode{
			ID:      uuid.New(),
			Network: "testnet",
		},
		OwnerID: "abhi",
	}
	userDev.IsRelayed = true
	userDev.RelayedBy = exitID.String()
	userDev.InternetGwID = exitID.String()
	userDev.SelectedInternetEgressID = "inet"
	require.True(t, IsUserOwnedDevice(userDev))
	require.True(t, nodeIsInternetExitClient(userDev))

	gw := &models.Node{
		CommonNode: models.CommonNode{
			ID:      gwID,
			Network: "testnet",
		},
	}
	assert.False(t, shouldRetainPeerDespiteRelay(userDev, gw, false, true, nil),
		"exit client must not keep a plain gateway as a direct peer; mesh goes via exit")

	exit := &models.Node{
		CommonNode: models.CommonNode{
			ID:      exitID,
			Network: "testnet",
		},
	}
	exitIDs := map[string]struct{}{exitID.String(): {}}
	assert.True(t, shouldRetainPeerDespiteRelay(userDev, exit, false, true, exitIDs),
		"exit client keeps the exit as a direct peer")
}

func TestInternetEgressRoutingNodeIDsFromList(t *testing.T) {
	exitA := uuid.New().String()
	exitB := uuid.New().String()
	got := InternetEgressRoutingNodeIDsFromList([]schema.Egress{
		{Status: true, Type: schema.EgressTypeInternet, Range: "*", Nodes: datatypes.JSONMap{exitA: true}},
		{Status: false, Type: schema.EgressTypeInternet, Range: "*", Nodes: datatypes.JSONMap{exitB: true}},
		{Status: true, Type: schema.EgressTypeCIDR, Range: "10.0.0.0/8", Nodes: datatypes.JSONMap{exitB: true}},
		{Status: true, Type: schema.EgressTypeInternet, Range: "*", Nodes: datatypes.JSONMap{exitB: true}},
	})
	assert.Equal(t, map[string]struct{}{exitA: {}, exitB: {}}, got)
}

func TestFilterConflictingEgressRoutesKeepsSpecificWhenNotExit(t *testing.T) {
	node := models.Node{
		CommonNode: models.CommonNode{ID: uuid.New(), Network: "testnet"},
	}
	peer := models.Node{
		CommonNode: models.CommonNode{ID: uuid.New(), Network: "testnet"},
	}
	peer.EgressDetails = models.EgressDetails{
		IsEgressGateway:     true,
		EgressGatewayRanges: []string{IPv4Network, "10.20.0.0/16", "10.10.0.0/16"},
		EgressGatewayRequest: models.EgressGatewayRequest{
			RangesWithMetric: []models.EgressRangeMetric{
				{Network: IPv4Network},
				{Network: "10.20.0.0/16"},
				{Network: "10.10.0.0/16"},
			},
		},
	}

	got := filterConflictingEgressRoutes(node, peer)
	assert.ElementsMatch(t, []string{"10.20.0.0/16", "10.10.0.0/16"}, got)
	assert.NotContains(t, got, IPv4Network)
}
