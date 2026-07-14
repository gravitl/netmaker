package logic

import (
	"context"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
)

// IsUserAllowedToJoinNetwork reports whether username may join the given network.
// network may be the network name (netid) or UUID.
var IsUserAllowedToJoinNetwork = defaultIsUserAllowedToJoinNetwork

func defaultIsUserAllowedToJoinNetwork(username, network string) bool {
	if username == "" || network == "" {
		return false
	}
	user := &schema.User{Username: username}
	if err := user.Get(db.WithContext(context.TODO())); err != nil {
		return false
	}

	// Resolve name vs UUID so group keys (network name) match path params that use either.
	networkName := network
	net := &schema.Network{ID: network, Name: network}
	if err := net.Get(db.WithContext(context.TODO())); err == nil && net.Name != "" {
		networkName = net.Name
	}

	// Network admins and any group network role use the canonical name.
	if UserHasNetworkGroupAccess(user, networkName) {
		return true
	}

	allNetworks, err := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return false
	}
	filtered := FilterNetworksByRole(allNetworks, user)
	for _, n := range filtered {
		if n.Name == networkName || n.Name == network || n.ID == network {
			return true
		}
	}
	return false
}
