package functions

import (
	"net/http"

	"github.com/gravitl/netmaker/schema"
)

// CreateNetwork - creates a network
func CreateNetwork(payload *schema.Network) *schema.Network {
	return request[schema.Network](http.MethodPost, "/api/networks", payload)
}

// GetNetworks - fetch all networks
func GetNetworks() *[]schema.Network {
	return request[[]schema.Network](http.MethodGet, "/api/networks", nil)
}

// GetNetwork - fetch a single network
func GetNetwork(name string) *schema.Network {
	return request[schema.Network](http.MethodGet, "/api/networks/"+name, nil)
}

// DeleteNetwork - delete a network
func DeleteNetwork(name string) *string {
	return request[string](http.MethodDelete, "/api/networks/"+name, nil)
}
