package functions

import (
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
	return request[models.User](http.MethodPut, "/api/users/networks/"+payload.UserName, payload)
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

func CreateUserRole(role models.UserRolePermissionTemplate) *models.SuccessResponse {
	return request[models.SuccessResponse](http.MethodPost, "/api/v1/users/role", role)
}
func UpdateUserRole(role models.UserRolePermissionTemplate) *models.SuccessResponse {
	return request[models.SuccessResponse](http.MethodPut, "/api/v1/users/role", role)
}

func ListUserRoles() *models.SuccessResponse {
	return request[models.SuccessResponse](http.MethodGet, "/api/v1/users/roles", nil)
}

func DeleteUserRole(roleID string) *models.SuccessResponse {
	return request[models.SuccessResponse](http.MethodDelete, fmt.Sprintf("/api/v1/users/role?role_id=%s", roleID), nil)
}
func GetUserRole(roleID string) *models.SuccessResponse {
	return request[models.SuccessResponse](http.MethodGet, fmt.Sprintf("/api/v1/users/role?role_id=%s", roleID), nil)
}

/*

	r.HandleFunc("/api/v1/users/roles", logic.SecurityCheck(true, http.HandlerFunc(listRoles))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/users/role", logic.SecurityCheck(true, http.HandlerFunc(getRole))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/users/role", logic.SecurityCheck(true, http.HandlerFunc(createRole))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/users/role", logic.SecurityCheck(true, http.HandlerFunc(updateRole))).Methods(http.MethodPut)
	r.HandleFunc("/api/v1/users/role", logic.SecurityCheck(true, http.HandlerFunc(deleteRole))).Methods(http.MethodDelete)
*/
