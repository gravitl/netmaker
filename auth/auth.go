package auth

import (
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/schema"
)

// == consts ==
const (
	node_signin_length = 64
)

func isUserIsAllowed(username, network string) (*schema.User, error) {

	user, err := logic.GetUser(username)
	if err != nil { // user must not exist, so try to make one
		return &schema.User{}, err
	}

	return user, nil
}
