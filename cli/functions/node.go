package functions

import (
	"fmt"
	"net/http"

	"github.com/gravitl/netmaker/models"
)

// GetNodes - fetch all nodes
func GetNodes(networkName ...string) *[]models.Node {
	if len(networkName) == 1 {
		return request[[]models.Node](http.MethodGet, "/api/nodes/"+networkName[0], nil)
	} else {
		return request[[]models.Node](http.MethodGet, "/api/nodes", nil)
	}
}

// GetNodeByID - fetch a single node by ID
func GetNodeByID(networkName, nodeID string) *models.NodeGet {
	return request[models.NodeGet](http.MethodGet, fmt.Sprintf("/api/nodes/%s/%s", networkName, nodeID), nil)
}

// UpdateNode - update a single node
func UpdateNode(networkName, nodeID string, node *models.Node) *models.Node {
	return request[models.Node](http.MethodPut, fmt.Sprintf("/api/nodes/%s/%s", networkName, nodeID), node)
}

// DeleteNode - delete a node
func DeleteNode(networkName, nodeID string) *models.SuccessResponse {
	return request[models.SuccessResponse](http.MethodDelete, fmt.Sprintf("/api/nodes/%s/%s", networkName, nodeID), nil)
}

// CreateRelay - turn a node into a relay
func CreateRelay(networkName, nodeID string, relayAddresses []string) *models.Node {
	return request[models.Node](http.MethodPost, fmt.Sprintf("/api/nodes/%s/%s/createrelay", networkName, nodeID), &models.RelayRequest{
		NetID:      networkName,
		NodeID:     nodeID,
		RelayAddrs: relayAddresses,
	})
}

// DeleteRelay - remove relay role from a node
func DeleteRelay(networkName, nodeID string) *models.Node {
	return request[models.Node](http.MethodDelete, fmt.Sprintf("/api/nodes/%s/%s/deleterelay", networkName, nodeID), nil)
}

// CreateEgress - turn a node into an egress
func CreateEgress(networkName, nodeID string, payload *models.EgressGatewayRequest) *models.Node {
	return request[models.Node](http.MethodPost, fmt.Sprintf("/api/nodes/%s/%s/creategateway", networkName, nodeID), payload)
}

// DeleteEgress - remove egress role from a node
func DeleteEgress(networkName, nodeID string) *models.Node {
	return request[models.Node](http.MethodDelete, fmt.Sprintf("/api/nodes/%s/%s/deletegateway", networkName, nodeID), nil)
}

// CreateIngress - turn a node into an ingress
func CreateIngress(networkName, nodeID string, failover bool) *models.Node {
	return request[models.Node](http.MethodPost, fmt.Sprintf("/api/nodes/%s/%s/createingress", networkName, nodeID), &struct {
		Failover bool `json:"failover"`
	}{Failover: failover})
}

// DeleteIngress - remove ingress role from a node
func DeleteIngress(networkName, nodeID string) *models.Node {
	return request[models.Node](http.MethodDelete, fmt.Sprintf("/api/nodes/%s/%s/deleteingress", networkName, nodeID), nil)
}

// UncordonNode - uncordon a node
func UncordonNode(networkName, nodeID string) *string {
	return request[string](http.MethodPost, fmt.Sprintf("/api/nodes/%s/%s/approve", networkName, nodeID), nil)
}
