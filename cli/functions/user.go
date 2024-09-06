package functions

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gravitl/netmaker/models"
)

// HasAdmin - check if server has an admin user
func HasAdmin() *bool {
	return request[bool](http.MethodGet, "/api/users/adm/hasadmin", nil)
}

// CreateUser - create a user
func CreateUser(payload *models.User) *models.User {
	return request[models.User](http.MethodPost, "/api/users/"+payload.UserName, payload)
}

// UpdateUser - update a user
func UpdateUser(payload *models.User) *models.User {
	return request[models.User](http.MethodPut, "/api/users/"+payload.UserName, payload)
}

// DeleteUser - delete a user
func DeleteUser(username string) *string {
	return request[string](http.MethodDelete, "/api/users/"+username, nil)
}

// GetUser - fetch a single user
func GetUser(username string) *models.User {
	return request[models.User](http.MethodGet, "/api/users/"+username, nil)
}

// ListUsers - fetch all users
func ListUsers() *[]models.ReturnUser {
	return request[[]models.ReturnUser](http.MethodGet, "/api/users", nil)
}

func ListUserRoles() (roles []models.UserRolePermissionTemplate) {
	resp := request[models.SuccessResponse](http.MethodGet, "/api/v1/users/roles", nil)
	d, _ := json.Marshal(resp.Response)
	json.Unmarshal(d, &roles)
	return
}

func DeleteUserRole(roleID string) *models.SuccessResponse {
	return request[models.SuccessResponse](http.MethodDelete, fmt.Sprintf("/api/v1/users/role?role_id=%s", roleID), nil)
}
func GetUserRole(roleID string) (role models.UserRolePermissionTemplate) {
	resp := request[models.SuccessResponse](http.MethodGet, fmt.Sprintf("/api/v1/users/role?role_id=%s", roleID), nil)
	d, _ := json.Marshal(resp.Response)
	json.Unmarshal(d, &role)
	return
}

func ListUserGrps() (groups []models.UserGroup) {
	resp := request[models.SuccessResponse](http.MethodGet, "/api/v1/users/groups", nil)
	d, _ := json.Marshal(resp.Response)
	json.Unmarshal(d, &groups)
	return
}

func DeleteUserGrp(grpID string) *models.SuccessResponse {
	return request[models.SuccessResponse](http.MethodDelete, fmt.Sprintf("/api/v1/users/group?group_id=%s", grpID), nil)
}

func GetUserGrp(grpID string) (group models.UserGroup) {
	resp := request[models.SuccessResponse](http.MethodGet, fmt.Sprintf("/api/v1/users/group?group_id=%s", grpID), nil)
	d, _ := json.Marshal(resp.Response)
	json.Unmarshal(d, &group)
	return
}
