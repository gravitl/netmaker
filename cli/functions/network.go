package functions

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gravitl/netmaker/models"
)

// CreateNetwork - creates a network
func CreateNetwork(payload *models.Network) *models.Network {
	return request[models.Network](http.MethodPost, "/api/networks", payload)
}

// UpdateNetwork - updates a network
func UpdateNetwork(name string, payload *models.Network) *models.Network {
	return request[models.Network](http.MethodPut, "/api/networks/"+url.QueryEscape(name), payload)
}

// UpdateNetworkNodeLimit - updates a network
func UpdateNetworkNodeLimit(name string, nodeLimit int32) *models.Network {
	return request[models.Network](http.MethodPut, fmt.Sprintf("/api/networks/%s/nodelimit", url.QueryEscape(name)), &models.Network{
		NodeLimit: nodeLimit,
	})
}

// GetNetworks - fetch all networks
func GetNetworks() *[]models.Network {
	return request[[]models.Network](http.MethodGet, "/api/networks", nil)
}

// GetNetwork - fetch a single network
func GetNetwork(name string) *models.Network {
	return request[models.Network](http.MethodGet, "/api/networks/"+url.QueryEscape(name), nil)
}

// DeleteNetwork - delete a network
func DeleteNetwork(name string) *string {
	return request[string](http.MethodDelete, "/api/networks/"+url.QueryEscape(name), nil)
}
