package logic

import (
	"context"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// NodeOwnerUsername returns the Netmaker username that owns this node, if any.
// Legacy ExtClient Active Users use StaticNode.OwnerID; registered desktop
// devices use OwnerID populated from Host.OwnerUsername.
func NodeOwnerUsername(n *models.Node) string {
	if n == nil {
		return ""
	}
	if n.OwnerID != "" {
		return n.OwnerID
	}
	if n.IsStatic || n.IsUserNode {
		return n.StaticNode.OwnerID
	}
	return n.StaticNode.OwnerID
}

// IsUserOwnedDevice reports whether this is a host-backed (non-ExtClient) device
// registered to a user. Runtime IsUserNode stays false for these so posture/ACL
// device paths remain intact; ownership is the user-policy subject signal.
func IsUserOwnedDevice(n *models.Node) bool {
	return n != nil && !n.IsStatic && !n.IsUserNode && NodeOwnerUsername(n) != ""
}

// isAllowedViaUserOwnership is true when either side is a user-owned subject
// whose user policies allow communication with the other peer.
func isAllowedViaUserOwnership(ctx context.Context, node, peer models.Node) bool {
	if owner := NodeOwnerUsername(&node); owner != "" {
		if ok, _ := IsUserAllowedToCommunicate(ctx, owner, peer); ok {
			return true
		}
	}
	if owner := NodeOwnerUsername(&peer); owner != "" {
		if ok, _ := IsUserAllowedToCommunicate(ctx, owner, node); ok {
			return true
		}
	}
	return false
}

// NodeHasEgressAccess reports whether the node may use egress e via device
// policies or (for user-owned devices) via that owner's user policies.
func NodeHasEgressAccess(ctx context.Context, node *models.Node, e *schema.Egress, deviceAcls []models.Acl) bool {
	if node == nil || e == nil {
		return false
	}
	if DoesNodeHaveAccessToEgress(node, e, deviceAcls) {
		return true
	}
	owner := NodeOwnerUsername(node)
	if owner == "" {
		return false
	}
	userDefault, err := GetDefaultPolicy(ctx, schema.NetworkID(node.Network), models.UserPolicy)
	if err == nil && userDefault.Enabled {
		return true
	}
	user := &schema.User{Username: owner}
	if err := user.GetWithMembership(ctx); err != nil {
		return false
	}
	return DoesUserHaveAccessToEgress(user, e, ListUserPolicies(ctx, schema.NetworkID(node.Network)))
}
