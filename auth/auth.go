package auth

import (
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"golang.org/x/oauth2"
)

// == consts ==
const (
	node_signin_length = 64
)

var (
	auth_provider *oauth2.Config
)

func isUserIsAllowed(username, network string) (*models.User, error) {

	user, err := logic.GetUser(username)
	if err != nil { // user must not exist, so try to make one
		return &models.User{}, err
	}

	return user, nil
}
