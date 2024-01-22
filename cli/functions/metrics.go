package functions

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gravitl/netmaker/models"
)

// GetNodeMetrics - fetch a single node's metrics
func GetNodeMetrics(networkName, nodeID string) *models.Metrics {
	return request[models.Metrics](http.MethodGet, fmt.Sprintf("/api/metrics/%s/%s", url.QueryEscape(networkName), url.QueryEscape(nodeID)), nil)
}

// GetNetworkNodeMetrics - fetch an entire network's metrics
func GetNetworkNodeMetrics(networkName string) *models.NetworkMetrics {
	return request[models.NetworkMetrics](http.MethodGet, "/api/metrics/"+url.QueryEscape(networkName), nil)
}

// GetAllMetrics - fetch all metrics
func GetAllMetrics() *models.NetworkMetrics {
	return request[models.NetworkMetrics](http.MethodGet, "/api/metrics", nil)
}

// GetNetworkExtMetrics - fetch external client metrics belonging to a network
func GetNetworkExtMetrics(networkName string) *map[string]models.Metric {
	return request[map[string]models.Metric](http.MethodGet, "/api/metrics-ext/"+url.QueryEscape(networkName), nil)
}
