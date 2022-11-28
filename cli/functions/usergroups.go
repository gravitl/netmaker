package functions

import (
	"net/http"

	"github.com/gravitl/netmaker/models/promodels"
)

func GetUsergroups() *promodels.UserGroups {
	return request[promodels.UserGroups](http.MethodGet, "/api/usergroups", nil)
}

func CreateUsergroup(usergroupName string) {
	request[any](http.MethodPost, "/api/usergroups/"+usergroupName, nil)
}

func DeleteUsergroup(usergroupName string) {
	request[any](http.MethodDelete, "/api/usergroups/"+usergroupName, nil)
}
