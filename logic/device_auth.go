package logic

import (
	"context"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
)

// IsUserAllowedToJoinNetwork reports whether username may join the given network.
// network may be the network name (netid) or UUID.
// ctx should carry tenant scope so User.Get can load PlatformRoleID and UserGroups
// from tenant_memberships_v1; if missing, DefaultScope is applied.
var IsUserAllowedToJoinNetwork = defaultIsUserAllowedToJoinNetwork

func defaultIsUserAllowedToJoinNetwork(ctx context.Context, username, network string) bool {
	if username == "" || network == "" {
		return false
	}
	if ctx == nil {
		ctx = context.TODO()
	}
	ctx = db.WithContext(ctx)
	if scope.ID(ctx) == "" {
		ctx = DefaultScope(ctx)
	}
	user := &schema.User{Username: username}
	if err := user.Get(ctx); err != nil {
		return false
	}
	return UserHasAccessToNetwork(ctx, user, network)
}

// UserHasAccessToNetwork reports whether the given user (with membership fields
// already loaded) may access/join the network. Prefer this when the caller
// already fetched the user under the correct tenant scope.
func UserHasAccessToNetwork(ctx context.Context, user *schema.User, network string) bool {
	if user == nil || user.Username == "" || network == "" {
		return false
	}
	if ctx == nil {
		ctx = context.TODO()
	}
	ctx = db.WithContext(ctx)

	// Resolve name vs UUID so group keys (network name) match path params that use either.
	networkName := network
	net := &schema.Network{ID: network, Name: network}
	if err := net.Get(ctx); err == nil && net.Name != "" {
		networkName = net.Name
	}

	// Network admins and any group network role use the canonical name.
	if UserHasNetworkGroupAccess(ctx, user, networkName) {
		return true
	}

	allNetworks, err := (&schema.Network{}).ListAll(ctx)
	if err != nil {
		return false
	}
	filtered := FilterNetworksByRole(ctx, allNetworks, user)
	for _, n := range filtered {
		if n.Name == networkName || n.Name == network || n.ID == network {
			return true
		}
	}
	return false
}

// UserHasDeviceNetworkWriteAccess reports whether the user may mutate device
// network membership/state (join, leave, exit-node selection). CE defaults to
// network access; Pro overrides with scope checks that deny read-only roles.
var UserHasDeviceNetworkWriteAccess = defaultUserHasDeviceNetworkWriteAccess

func defaultUserHasDeviceNetworkWriteAccess(ctx context.Context, user *schema.User, network string) bool {
	return UserHasAccessToNetwork(ctx, user, network)
}
