package functions

import (
	"net/http"

	"github.com/gravitl/netmaker/models/promodels"
)

// GetUsergroups - fetch all usergroups
func GetUsergroups() *promodels.UserGroups {
	return request[promodels.UserGroups](http.MethodGet, "/api/usergroups", nil)
}

// CreateUsergroup - create a usergroup
func CreateUsergroup(usergroupName string) {
	request[any](http.MethodPost, "/api/usergroups/"+usergroupName, nil)
}

// DeleteUsergroup - delete a usergroup
func DeleteUsergroup(usergroupName string) {
	request[any](http.MethodDelete, "/api/usergroups/"+usergroupName, nil)
}
