package functions

import (
	"net/http"

	"github.com/gravitl/netmaker/models"
)

// CreateNetwork - creates a network
func CreateNetwork(payload *models.Network) *models.Network {
	return request[models.Network](http.MethodPost, "/api/networks", payload)
}

// GetNetworks - fetch all networks
func GetNetworks() *[]models.Network {
	return request[[]models.Network](http.MethodGet, "/api/networks", nil)
}

// GetNetwork - fetch a single network
func GetNetwork(name string) *models.Network {
	return request[models.Network](http.MethodGet, "/api/networks/"+name, nil)
}

// DeleteNetwork - delete a network
func DeleteNetwork(name string) *string {
	return request[string](http.MethodDelete, "/api/networks/"+name, nil)
}
