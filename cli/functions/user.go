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

func ListUsers() *[]models.ReturnUser {
	return request[[]models.ReturnUser](http.MethodGet, "/api/users", nil)
}
