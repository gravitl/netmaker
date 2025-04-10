package functions

import (
	"fmt"
	"net/http"

	"github.com/gravitl/netmaker/models"
)

// GetAllExtClients - fetch all external clients
func GetAllExtClients() *[]models.ExtClient {
	return request[[]models.ExtClient](http.MethodGet, "/api/extclients", nil)
}

// GetNetworkExtClients - fetch external clients associated with a network
func GetNetworkExtClients(networkName string) *[]models.ExtClient {
	return request[[]models.ExtClient](http.MethodGet, "/api/extclients/"+networkName, nil)
}

// GetExtClient - fetch a single external client
func GetExtClient(networkName, clientID string) *models.ExtClient {
	return request[models.ExtClient](http.MethodGet, fmt.Sprintf("/api/extclients/%s/%s", networkName, clientID), nil)
}

// GetExtClientConfig - fetch a wireguard config of an external client
func GetExtClientConfig(networkName, clientID string) string {
	return get(fmt.Sprintf("/api/extclients/%s/%s/file", networkName, clientID))
}

// GetExtClientConfig - auto fetch a client config
func GetExtClientHAConfig(networkName string) string {
	return get(fmt.Sprintf("/api/v1/client_conf/%s", networkName))
}

// CreateExtClient - create an external client
func CreateExtClient(networkName, nodeID string, extClient models.CustomExtClient) {
	request[any](http.MethodPost, fmt.Sprintf("/api/extclients/%s/%s", networkName, nodeID), extClient)
}

// DeleteExtClient - delete an external client
func DeleteExtClient(networkName, clientID string) *models.SuccessResponse {
	return request[models.SuccessResponse](http.MethodDelete, fmt.Sprintf("/api/extclients/%s/%s", networkName, clientID), nil)
}

// UpdateExtClient - update an external client
func UpdateExtClient(networkName, clientID string, payload *models.CustomExtClient) *models.ExtClient {
	return request[models.ExtClient](http.MethodPut, fmt.Sprintf("/api/extclients/%s/%s", networkName, clientID), payload)
}
