package functions

import (
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

// UpdateHostNetworks - update a host's networks
func UpdateHostNetworks(hostID string, networks []string) *hostNetworksUpdatePayload {
	return request[hostNetworksUpdatePayload](http.MethodPut, "/api/hosts/"+hostID+"/networks", &hostNetworksUpdatePayload{
		Networks: networks,
	})
}
