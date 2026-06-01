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

func isUserAllowed(username, network string) bool {
	user := &schema.User{Username: username}
	err := user.Get(db.WithContext(context.TODO()))
	if err != nil {
		return false
	}

	if user.PlatformRoleID == schema.SuperAdminRole || user.PlatformRoleID == schema.AdminRole {
		return true
	}

	return false
}
