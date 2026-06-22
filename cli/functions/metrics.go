package functions

import (
	"fmt"
	"net/http"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// GetNodeMetrics - fetch a single node's metrics
func GetNodeMetrics(networkName, nodeID string) *schema.Metrics {
	return request[schema.Metrics](http.MethodGet, fmt.Sprintf("/api/metrics/%s/%s", networkName, nodeID), nil)
}

// GetNetworkNodeMetrics - fetch an entire network's metrics
func GetNetworkNodeMetrics(networkName string) *models.NetworkMetrics {
	return request[models.NetworkMetrics](http.MethodGet, "/api/metrics/"+networkName, nil)
}

// GetAllMetrics - fetch all metrics
func GetAllMetrics() *models.NetworkMetrics {
	return request[models.NetworkMetrics](http.MethodGet, "/api/metrics", nil)
}

// GetNetworkExtMetrics - fetch external client metrics belonging to a network
func GetNetworkExtMetrics(networkName string) *map[string]schema.Metric {
	return request[map[string]schema.Metric](http.MethodGet, "/api/metrics-ext/"+networkName, nil)
}
