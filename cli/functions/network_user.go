package functions

import (
	"fmt"
	"net/http"

	"github.com/gravitl/netmaker/ee/ee_controllers"
	"github.com/gravitl/netmaker/models/promodels"
)

func GetAllNetworkUsers() *map[string][]promodels.NetworkUser {
	return request[map[string][]promodels.NetworkUser](http.MethodGet, "/api/networkusers", nil)
}

func GetNetworkUsers(networkName string) *promodels.NetworkUserMap {
	return request[promodels.NetworkUserMap](http.MethodGet, "/api/networkusers/"+networkName, nil)
}

func GetNetworkUser(networkName, networkUserName string) *promodels.NetworkUser {
	return request[promodels.NetworkUser](http.MethodGet, fmt.Sprintf("/api/networkusers/%s/%s", networkName, networkUserName), nil)
}

func CreateNetworkUser(networkName string, payload *promodels.NetworkUser) {
	request[any](http.MethodPost, "/api/networkusers/"+networkName, payload)
}

func UpdateNetworkUser(networkName string, payload *promodels.NetworkUser) {
	request[any](http.MethodPut, "/api/networkusers/"+networkName, payload)
}

func GetNetworkUserData(networkUserName string) *ee_controllers.NetworkUserDataMap {
	return request[ee_controllers.NetworkUserDataMap](http.MethodGet, fmt.Sprintf("/api/networkusers/data/%s/me", networkUserName), nil)
}

func DeleteNetworkUser(networkName, networkUserName string) {
	request[any](http.MethodDelete, fmt.Sprintf("/api/networkusers/%s/%s", networkName, networkUserName), nil)
}
