package functions

import (
	"net/http"
	"net/url"

	"github.com/gravitl/netmaker/models"
)

// HasAdmin - check if server has an admin user
func HasAdmin() *bool {
	return request[bool](http.MethodGet, "/api/users/adm/hasadmin", nil)
}

// CreateUser - create a user
func CreateUser(payload *models.User) *models.User {
	return request[models.User](http.MethodPost, "/api/users/"+url.QueryEscape(payload.UserName), payload)
}

// UpdateUser - update a user
func UpdateUser(payload *models.User) *models.User {
	return request[models.User](http.MethodPut, "/api/users/networks/"+url.QueryEscape(payload.UserName), payload)
}

// DeleteUser - delete a user
func DeleteUser(username string) *string {
	return request[string](http.MethodDelete, "/api/users/"+url.QueryEscape(username), nil)
}

// GetUser - fetch a single user
func GetUser(username string) *models.User {
	return request[models.User](http.MethodGet, "/api/users/"+url.QueryEscape(username), nil)
}

// ListUsers - fetch all users
func ListUsers() *[]models.ReturnUser {
	return request[[]models.ReturnUser](http.MethodGet, "/api/users", nil)
}
