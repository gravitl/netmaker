package functions

import (
	"fmt"
	"net/http"

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
func DeleteHost(hostID string) *models.ApiHost {
	return request[models.ApiHost](http.MethodDelete, "/api/hosts/"+hostID, nil)
}

// UpdateHost - update a host
func UpdateHost(hostID string, body *models.ApiHost) *models.ApiHost {
	return request[models.ApiHost](http.MethodPut, "/api/hosts/"+hostID, body)
}

// AddHostToNetwork - add a network to host
func AddHostToNetwork(hostID, network string) *hostNetworksUpdatePayload {
	return request[hostNetworksUpdatePayload](http.MethodPost, "/api/hosts/"+hostID+"/networks/"+network, nil)
}

// DeleteHostFromNetwork - deletes a network from host
func DeleteHostFromNetwork(hostID, network string) *hostNetworksUpdatePayload {
	return request[hostNetworksUpdatePayload](http.MethodDelete, "/api/hosts/"+hostID+"/networks/"+network, nil)
}

// CreateRelay - turn a host into a relay
func CreateRelay(hostID string, relayedHosts []string) *models.ApiHost {
	return request[models.ApiHost](http.MethodPost, fmt.Sprintf("/api/hosts/%s/relay", hostID), &models.HostRelayRequest{
		HostID:       hostID,
		RelayedHosts: relayedHosts,
	})
}

// DeleteRelay - remove relay role from a host
func DeleteRelay(hostID string) *models.ApiHost {
	return request[models.ApiHost](http.MethodDelete, fmt.Sprintf("/api/hosts/%s/relay", hostID), nil)
}

// RefreshKeys - refresh wireguard keys
func RefreshKeys(hostID string) any {
	if hostID == "" {
		return request[any](http.MethodPost, "/api/hosts/keys", nil)
	}
	return request[any](http.MethodPost, fmt.Sprintf("/api/hosts/%s/keys", hostID), nil)

}
