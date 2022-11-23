package functions

import (
	"fmt"
	"net/http"

	"github.com/gravitl/netmaker/models"
)

func GetAllExtClients() *[]models.ExtClient {
	return request[[]models.ExtClient](http.MethodGet, "/api/extclients", nil)
}

func GetNetworkExtClients(networkName string) *[]models.ExtClient {
	return request[[]models.ExtClient](http.MethodGet, "/api/extclients/"+networkName, nil)
}

func GetExtClient(networkName, clientID string) *models.ExtClient {
	return request[models.ExtClient](http.MethodGet, fmt.Sprintf("/api/extclients/%s/%s", networkName, clientID), nil)
}

func GetExtClientConfig(networkName, clientID, configType string) *models.ExtClient {
	return request[models.ExtClient](http.MethodGet, fmt.Sprintf("/api/extclients/%s/%s/%s", networkName, clientID, configType), nil)
}

func CreateExtClient(networkName, nodeID, extClientID string) {
	if extClientID != "" {
		request[any](http.MethodPost, fmt.Sprintf("/api/extclients/%s/%s", networkName, nodeID), &models.CustomExtClient{
			ClientID: extClientID,
		})
	} else {
		request[any](http.MethodPost, fmt.Sprintf("/api/extclients/%s/%s", networkName, nodeID), nil)
	}
}

func DeleteExtClient(networkName, clientID string) *models.SuccessResponse {
	return request[models.SuccessResponse](http.MethodDelete, fmt.Sprintf("/api/extclients/%s/%s", networkName, clientID), nil)
}
