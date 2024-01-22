package functions

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gravitl/netmaker/models"
)

type hostNetworksUpdatePayload struct {
	Networks []string `json:"networks"`
}

// GetHosts - fetch all host entries
func GetHosts() *[]models.ApiHost {
	return request[[]models.ApiHost](http.MethodGet, "/api/hosts", nil)
}

// DeleteHost - delete a host
func DeleteHost(hostID string, force bool) *models.ApiHost {
	return request[models.ApiHost](http.MethodDelete, fmt.Sprintf("/api/hosts/%s?force=%t", url.QueryEscape(hostID), force), nil)
}

// UpdateHost - update a host
func UpdateHost(hostID string, body *models.ApiHost) *models.ApiHost {
	return request[models.ApiHost](http.MethodPut, "/api/hosts/"+url.QueryEscape(hostID), body)
}

// AddHostToNetwork - add a network to host
func AddHostToNetwork(hostID, network string) *hostNetworksUpdatePayload {
	return request[hostNetworksUpdatePayload](http.MethodPost, "/api/hosts/"+url.QueryEscape(hostID)+"/networks/"+url.QueryEscape(network), nil)
}

// DeleteHostFromNetwork - deletes a network from host
func DeleteHostFromNetwork(hostID, network string) *hostNetworksUpdatePayload {
	return request[hostNetworksUpdatePayload](http.MethodDelete, "/api/hosts/"+url.QueryEscape(hostID)+"/networks/"+url.QueryEscape(network), nil)
}

// CreateRelay - add relay to a node
func CreateRelay(netID, nodeID string, relayedNodes []string) *models.ApiNode {
	return request[models.ApiNode](http.MethodPost, fmt.Sprintf("/api/nodes/%s/%s/createrelay", url.QueryEscape(netID), url.QueryEscape(nodeID)), &models.RelayRequest{
		NodeID:       nodeID,
		NetID:        netID,
		RelayedNodes: relayedNodes,
	})
}

// DeleteRelay - remove relay from a node
func DeleteRelay(netID, nodeID string) *models.ApiNode {
	return request[models.ApiNode](http.MethodDelete, fmt.Sprintf("/api/nodes/%s/%s/deleterelay", url.QueryEscape(netID), url.QueryEscape(nodeID)), nil)
}

// RefreshKeys - refresh wireguard keys
func RefreshKeys(hostID string) any {
	if hostID == "" {
		return request[any](http.MethodPut, "/api/hosts/keys", nil)
	}
	return request[any](http.MethodPut, fmt.Sprintf("/api/hosts/%s/keys", url.QueryEscape(hostID)), nil)

}
