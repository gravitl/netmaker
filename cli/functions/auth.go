package functions

import (
	"net/http"

	"github.com/gravitl/netmaker/models"
)

func LoginWithUserAndPassword(username, password string) *models.SuccessResponse {
	authParams := &models.UserAuthParams{UserName: username, Password: password}
	return Request[models.SuccessResponse](http.MethodPost, "/api/users/adm/authenticate", authParams)
}
