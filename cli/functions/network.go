package functions

import (
	"net/http"

	"github.com/gravitl/netmaker/models"
)

// CreateNetwork - creates a network
func CreateNetwork(payload *models.Network) *models.Network {
	return Request[models.Network](http.MethodPost, "/api/networks", payload)
}

// GetNetworks - fetch all networks
func GetNetworks() *models.Network {
	return Request[models.Network](http.MethodGet, "/api/networks", nil)
}
