package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/assert"
	"gorm.io/datatypes"
)

func TestDeduplicateEgressRoutesMergesRangesForSamePeerAndNetwork(t *testing.T) {
	routes := []models.EgressNetworkRoutes{
		{
			PeerKey:      "relay-peer-key",
			Network:      "testnet",
			EgressRanges: []string{"10.100.0.0/24"},
			EgressRangesWithMetric: []models.EgressRangeMetric{
				{
					Network:     "10.100.0.0/24",
					RouteMetric: 100,
					Nat:         true,
					Mode:        "direct",
				},
			},
			EgressGwAddr: net.IPNet{IP: net.ParseIP("10.0.0.2"), Mask: net.CIDRMask(32, 32)},
		},
		{
			PeerKey:      "relay-peer-key",
			Network:      "testnet",
			EgressRanges: []string{"10.200.0.0/24", "10.100.0.0/24"},
			EgressRangesWithMetric: []models.EgressRangeMetric{
				{
					Network:     "10.200.0.0/24",
					RouteMetric: 200,
					Nat:         true,
					Mode:        "direct",
				},
				{
					Network:     "10.100.0.0/24",
					RouteMetric: 100,
					Nat:         true,
					Mode:        "direct",
				},
			},
			EgressGwAddr: net.IPNet{IP: net.ParseIP("10.0.0.3"), Mask: net.CIDRMask(32, 32)},
		},
		{
			PeerKey:      "different-peer-key",
			Network:      "testnet",
			EgressRanges: []string{"10.250.0.0/24"},
		},
	}

	deduped := deduplicateEgressRoutes(routes)

	assert.Len(t, deduped, 2)

	var merged models.EgressNetworkRoutes
	for _, route := range deduped {
		if route.PeerKey == "relay-peer-key" && route.Network == "testnet" {
			merged = route
			break
		}
	}

	assert.Equal(t, []string{"10.100.0.0/24", "10.200.0.0/24"}, merged.EgressRanges)
	assert.Len(t, merged.EgressRangesWithMetric, 2)
	assert.ElementsMatch(t, []models.EgressRangeMetric{
		{
			Network:     "10.100.0.0/24",
			RouteMetric: 100,
			Nat:         true,
			Mode:        "direct",
		},
		{
			Network:     "10.200.0.0/24",
			RouteMetric: 200,
			Nat:         true,
			Mode:        "direct",
		},
	}, merged.EgressRangesWithMetric)
}

func TestWithoutDefaultRoutesStripsFullTunnel(t *testing.T) {
	_, v4, err := net.ParseCIDR(IPv4Network)
	assert.NoError(t, err)
	_, v6, err := net.ParseCIDR(IPv6Network)
	assert.NoError(t, err)
	_, lan, err := net.ParseCIDR("10.100.0.0/24")
	assert.NoError(t, err)
	_, host, err := net.ParseCIDR("10.0.0.5/32")
	assert.NoError(t, err)

	got := withoutDefaultRoutes([]net.IPNet{*host, *v4, *lan, *v6})
	assert.ElementsMatch(t, []net.IPNet{*host, *lan}, got)
}

func TestGetAllowedIPsOmitsDefaultRoutesUnlessExitClient(t *testing.T) {
	gwID := uuid.New()
	clientID := uuid.New()

	gw := models.Node{
		CommonNode: models.CommonNode{
			ID:      gwID,
			Network: "testnet",
			Address: net.IPNet{IP: net.ParseIP("10.0.0.1"), Mask: net.CIDRMask(32, 32)},
			// IsRelay intentionally false so GetAllowedIpsForRelayed (DB) is not invoked.
			IsGw: true,
		},
	}
	gw.EgressDetails = models.EgressDetails{
		IsEgressGateway: true,
		EgressGatewayRanges: []string{
			IPv4Network,
			IPv6Network,
			"10.100.0.0/24",
		},
		EgressGatewayRequest: models.EgressGatewayRequest{
			Ranges: []string{IPv4Network, IPv6Network, "10.100.0.0/24"},
			RangesWithMetric: []models.EgressRangeMetric{
				{Network: IPv4Network},
				{Network: IPv6Network},
				{Network: "10.100.0.0/24"},
			},
		},
	}

	// Peer of an exit/auto-relay gateway that is NOT using the exit node.
	client := models.Node{
		CommonNode: models.CommonNode{
			ID:      clientID,
			Network: "testnet",
			Address: net.IPNet{IP: net.ParseIP("10.0.0.5"), Mask: net.CIDRMask(32, 32)},
		},
	}

	ctx := db.WithContext(context.TODO())
	allowed := GetAllowedIPs(ctx, &client, &gw, nil)
	hasLAN := false
	for _, ip := range allowed {
		assert.NotEqual(t, IPv4Network, ip.String(), "non-exit client must not get default v4 route")
		assert.NotEqual(t, IPv6Network, ip.String(), "non-exit client must not get default v6 route")
		if ip.String() == "10.100.0.0/24" {
			hasLAN = true
		}
	}
	assert.True(t, hasLAN, "non-default egress ranges should still be advertised")

	// Exit client via legacy InternetGwID assignment.
	exitClient := client
	exitClient.InternetGwID = gwID.String()
	exitAllowed := GetAllowedIPs(ctx, &exitClient, &gw, nil)
	hasV4 := false
	for _, ip := range exitAllowed {
		if ip.String() == IPv4Network {
			hasV4 = true
		}
	}
	assert.True(t, hasV4, "exit client should get 0.0.0.0/0")
}

func TestAllowedIPsFromRelayedNodeIncludesNonDefaultEgress(t *testing.T) {
	relayed := models.Node{
		CommonNode: models.CommonNode{
			ID:      uuid.New(),
			Network: "testnet",
			Address: net.IPNet{IP: net.ParseIP("10.0.0.9"), Mask: net.CIDRMask(32, 32)},
		},
	}
	relayed.EgressDetails = models.EgressDetails{
		IsEgressGateway: true,
		EgressGatewayRanges: []string{
			IPv4Network,
			IPv6Network,
			"10.50.0.0/24",
		},
	}

	got := withoutDefaultRoutes(allowedIPsFromRelayedNode(&relayed))
	hasOverlay := false
	hasLAN := false
	for _, ip := range got {
		assert.NotEqual(t, IPv4Network, ip.String(), "default v4 route must not be advertised via relay/exit")
		assert.NotEqual(t, IPv6Network, ip.String(), "default v6 route must not be advertised via relay/exit")
		if ip.String() == "10.0.0.9/32" {
			hasOverlay = true
		}
		if ip.String() == "10.50.0.0/24" {
			hasLAN = true
		}
	}
	assert.True(t, hasOverlay, "relayed client overlay IP should be advertised")
	assert.True(t, hasLAN, "relayed client LAN egress should be advertised on peers' wg AllowedIPs")
}

func TestGetAllowedIPsNonGwExitAdvertisesRelayedClientRanges(t *testing.T) {
	originalGetNodeByID := getNodeByID
	originalListEgress := listEgressByNetwork
	t.Cleanup(func() {
		getNodeByID = originalGetNodeByID
		listEgressByNetwork = originalListEgress
	})

	exitID := uuid.New()
	observerID := uuid.New()
	clientID := uuid.New()

	exitClient := models.Node{
		CommonNode: models.CommonNode{
			ID:      clientID,
			Network: "testnet",
			Address: net.IPNet{IP: net.ParseIP("10.0.0.9"), Mask: net.CIDRMask(32, 32)},
		},
	}
	exitNode := models.Node{
		CommonNode: models.CommonNode{
			ID:           exitID,
			Network:      "testnet",
			Address:      net.IPNet{IP: net.ParseIP("10.0.0.1"), Mask: net.CIDRMask(32, 32)},
			IsGw:         false,
			IsRelay:      false,
			RelayedNodes: []string{clientID.String()},
		},
	}
	observer := models.Node{
		CommonNode: models.CommonNode{
			ID:      observerID,
			Network: "testnet",
			Address: net.IPNet{IP: net.ParseIP("10.0.0.5"), Mask: net.CIDRMask(32, 32)},
		},
	}

	getNodeByID = func(id string) (models.Node, error) {
		if id == clientID.String() {
			return exitClient, nil
		}
		return models.Node{}, fmt.Errorf("not found")
	}
	listEgressByNetwork = func(ctx context.Context, network string) ([]schema.Egress, error) {
		return []schema.Egress{{
			ID:      "lan-egress",
			Network: network,
			Status:  true,
			Range:   "10.50.0.0/24",
			Nodes:   datatypes.JSONMap{clientID.String(): json.Number("100")},
		}}, nil
	}

	assert.True(t, peerRelaysExitClients(&exitNode), "non-gw exit with RelayedNodes should advertise exit clients")
	assert.False(t, usesPeerAsInternetExit(&observer, &exitNode), "observer is not an exit client")

	ctx := db.WithContext(context.TODO())
	allowed := GetAllowedIPs(ctx, &observer, &exitNode, nil)
	hasOverlay := false
	hasLAN := false
	for _, ip := range allowed {
		assert.NotEqual(t, IPv4Network, ip.String(), "non-exit observer must not get default v4 via non-gw exit")
		assert.NotEqual(t, IPv6Network, ip.String(), "non-exit observer must not get default v6 via non-gw exit")
		if ip.String() == "10.0.0.9/32" {
			hasOverlay = true
		}
		if ip.String() == "10.50.0.0/24" {
			hasLAN = true
		}
	}
	assert.True(t, hasOverlay, "exit client's overlay IP should appear on peers' wg AllowedIPs")
	assert.True(t, hasLAN, "exit client's egress range should appear on peers' wg AllowedIPs when exit is not a GW")
}
