package logic

import (
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
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

	allowed := GetAllowedIPs(&client, &gw, nil)
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
	exitAllowed := GetAllowedIPs(&exitClient, &gw, nil)
	hasV4 := false
	for _, ip := range exitAllowed {
		if ip.String() == IPv4Network {
			hasV4 = true
		}
	}
	assert.True(t, hasV4, "exit client should get 0.0.0.0/0")
}

