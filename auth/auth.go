package auth

import (
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

// == consts ==
const (
	node_signin_length = 64
)

func isUserIsAllowed(username, network string) (*models.User, error) {

	user, err := logic.GetUser(username)
	if err != nil { // user must not exist, so try to make one
		return &models.User{}, err
	}

	return user, nil
}
