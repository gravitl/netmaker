package functions

import (
	"net/http"

	"github.com/gravitl/netmaker/models"
)

// CreateNetwork - creates a network
func CreateNetwork(payload *models.Network) *models.Network {
	return request[models.Network](http.MethodPost, "/api/networks", payload)
}

// UpdateNetwork - updates a network
func UpdateNetwork(name string, payload *models.Network) *models.Network {
	return request[models.Network](http.MethodPut, "/api/networks/"+name, payload)
}

// GetNetworks - fetch all networks
func GetNetworks() *[]models.Network {
	return request[[]models.Network](http.MethodGet, "/api/networks", nil)
}

// GetNetwork - fetch a single network
func GetNetwork(name string) *models.Network {
	return request[models.Network](http.MethodGet, "/api/networks/"+name, nil)
}
