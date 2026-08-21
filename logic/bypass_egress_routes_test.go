package logic

import (
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.True(t, shouldRetainPeerDespiteRelay(client, exit, false))
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
	assert.False(t, shouldRetainPeerDespiteRelay(client, site, false),
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
	assert.False(t, shouldRetainPeerDespiteRelay(client, meshPeer, false))
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
	assert.False(t, shouldRetainPeerDespiteRelay(site, client, false),
		"without resolvable BypassEgressRoutes on client, GW must not retain")
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
