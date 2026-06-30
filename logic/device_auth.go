package logic

import (
	"context"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
)

// IsUserAllowedToJoinNetwork reports whether username may join the given network.
var IsUserAllowedToJoinNetwork = defaultIsUserAllowedToJoinNetwork

func defaultIsUserAllowedToJoinNetwork(username, network string) bool {
	if username == "" || network == "" {
		return false
	}
	user := &schema.User{Username: username}
	if err := user.Get(db.WithContext(context.TODO())); err != nil {
		return false
	}
	allNetworks, err := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return false
	}
	filtered := FilterNetworksByRole(allNetworks, user)
	for _, net := range filtered {
		if net.Name == network {
			return true
		}
	}
	return false
}
