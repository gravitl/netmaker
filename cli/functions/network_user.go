package functions

import (
	"fmt"
	"net/http"

	"github.com/gravitl/netmaker/ee/ee_controllers"
	"github.com/gravitl/netmaker/models/promodels"
)

// GetAllNetworkUsers - fetch all network users
func GetAllNetworkUsers() *map[string][]promodels.NetworkUser {
	return request[map[string][]promodels.NetworkUser](http.MethodGet, "/api/networkusers", nil)
}

// GetNetworkUsers - fetch network users belonging to a particular network
func GetNetworkUsers(networkName string) *promodels.NetworkUserMap {
	return request[promodels.NetworkUserMap](http.MethodGet, "/api/networkusers/"+networkName, nil)
}

// GetNetworkUser - fetch a single network user
func GetNetworkUser(networkName, networkUserName string) *promodels.NetworkUser {
	return request[promodels.NetworkUser](http.MethodGet, fmt.Sprintf("/api/networkusers/%s/%s", networkName, networkUserName), nil)
}

// CreateNetworkUser - create a network user
func CreateNetworkUser(networkName string, payload *promodels.NetworkUser) {
	request[any](http.MethodPost, "/api/networkusers/"+networkName, payload)
}

// UpdateNetworkUser - update a network user
func UpdateNetworkUser(networkName string, payload *promodels.NetworkUser) {
	request[any](http.MethodPut, "/api/networkusers/"+networkName, payload)
}

// GetNetworkUserData - fetch a network user's complete data
func GetNetworkUserData(networkUserName string) *ee_controllers.NetworkUserDataMap {
	return request[ee_controllers.NetworkUserDataMap](http.MethodGet, fmt.Sprintf("/api/networkusers/data/%s/me", networkUserName), nil)
}

// DeleteNetworkUser - delete a network user
func DeleteNetworkUser(networkName, networkUserName string) {
	request[any](http.MethodDelete, fmt.Sprintf("/api/networkusers/%s/%s", networkName, networkUserName), nil)
}
