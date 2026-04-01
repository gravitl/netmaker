package auth

import (
	"context"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
)

// == consts ==
const (
	node_signin_length = 64
)

func isUserIsAllowed(username, network string) (*schema.User, error) {

	user := &schema.User{Username: username}
	err := user.Get(db.WithContext(context.TODO()))
	if err != nil { // user must not exist, so try to make one
		return &schema.User{}, err
	}

	return user, nil
}
