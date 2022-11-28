package functions

import (
	"net/http"

	"github.com/gravitl/netmaker/models"
)

func HasAdmin() *bool {
	return request[bool](http.MethodGet, "/api/users/adm/hasadmin", nil)
}

func CreateUser(payload *models.User) *models.User {
	return request[models.User](http.MethodPost, "/api/users/"+payload.UserName, payload)
}

func UpdateUser(payload *models.User) *models.User {
	return request[models.User](http.MethodPut, "/api/users/networks/"+payload.UserName, payload)
}

func DeleteUser(username string) *string {
	return request[string](http.MethodDelete, "/api/users/"+username, nil)
}

func GetUser(username string) *models.User {
	return request[models.User](http.MethodGet, "/api/users/"+username, nil)
}

func ListUsers() *[]models.ReturnUser {
	return request[[]models.ReturnUser](http.MethodGet, "/api/users", nil)
}
