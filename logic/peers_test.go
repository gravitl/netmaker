package logic

import (
	"net"
	"testing"

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
