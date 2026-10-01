package logic

import (
	"context"
	"errors"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// ErrUserDeviceInfrastructureRole is returned when a user-registered device is
// assigned a gateway, relay, auto-relay, or egress routing role.
var ErrUserDeviceInfrastructureRole = errors.New("user-registered devices cannot be set as gateways, relays, or egress routing nodes")

// IsUserOwnedHost reports whether the host was registered by an end user
// (desktop/device flow). Admin dashboard must not link these into networks;
// the owning user joins via device APIs instead.
func IsUserOwnedHost(h *schema.Host) bool {
	return h != nil && h.OwnerUsername != ""
}

// UserDevicesAreNotPeers reports whether two hosts must not form a WireGuard peer.
// User-registered devices mesh with infrastructure, not with other user devices.
func UserDevicesAreNotPeers(a, b *schema.Host) bool {
	if a == nil || b == nil || a.ID == b.ID {
		return false
	}
	return IsUserOwnedHost(a) && IsUserOwnedHost(b)
}

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

// ErrUserOwnedNodeInfrastructureRole rejects infrastructure roles on a
// host-backed user device. Legacy ExtClient user nodes are not included.
func ErrUserOwnedNodeInfrastructureRole(n *models.Node) error {
	if IsUserOwnedDevice(n) {
		return ErrUserDeviceInfrastructureRole
	}
	return nil
}

// ErrUserOwnedHostInfrastructureRole rejects infrastructure roles on a
// user-registered host.
func ErrUserOwnedHostInfrastructureRole(h *schema.Host) error {
	if IsUserOwnedHost(h) {
		return ErrUserDeviceInfrastructureRole
	}
	return nil
}

// ErrUserDeviceGainingInfrastructureRole rejects a node update that promotes a
// user-registered device into a gateway, relay, auto-relay, or egress role,
// including membership in a tag that already routes egress.
func ErrUserDeviceGainingInfrastructureRole(ctx context.Context, current, next *models.Node) error {
	if next == nil || (!IsUserOwnedDevice(current) && !IsUserOwnedDevice(next)) {
		return nil
	}
	if userDeviceInfrastructureRoleTurnedOn(current, next) {
		return ErrUserDeviceInfrastructureRole
	}
	if current != nil && !sameTagSet(current.Tags, next.Tags) {
		return errIfUserDeviceAssignedEgressTags(ctx, next.Network, next.Tags)
	}
	return nil
}

// ErrTaggingUserDevicesAsEgressRouters rejects adding user-registered devices
// to a tag that is already used as an egress routing set.
func ErrTaggingUserDevicesAsEgressRouters(ctx context.Context, tagID models.TagID, network string, nodes []models.ApiNode) error {
	if tagID == "" || network == "" || len(nodes) == 0 {
		return nil
	}
	hasUserDevice := false
	for i := range nodes {
		if nodes[i].IsStatic || nodes[i].ID == "" {
			continue
		}
		node, err := GetNodeByID(nodes[i].ID)
		if err != nil {
			continue
		}
		if IsUserOwnedDevice(&node) {
			hasUserDevice = true
			break
		}
	}
	if !hasUserDevice {
		return nil
	}
	return errIfEgressUsesTag(ctx, network, tagID)
}

func userDeviceInfrastructureRoleTurnedOn(current, next *models.Node) bool {
	curGw := current != nil && (current.IsGw || current.IsIngressGateway)
	curRelay := current != nil && current.IsRelay
	curInet := current != nil && current.IsInternetGateway
	curAuto := current != nil && current.IsAutoRelay
	curEgress := current != nil && current.EgressDetails.IsEgressGateway
	if !curGw && (next.IsGw || next.IsIngressGateway) {
		return true
	}
	if !curRelay && next.IsRelay {
		return true
	}
	if !curInet && next.IsInternetGateway {
		return true
	}
	if !curAuto && next.IsAutoRelay {
		return true
	}
	if !curEgress && next.EgressDetails.IsEgressGateway {
		return true
	}
	return false
}

func sameTagSet(a, b map[models.TagID]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for id := range a {
		if _, ok := b[id]; !ok {
			return false
		}
	}
	return true
}

func errIfUserDeviceAssignedEgressTags(ctx context.Context, network string, tags map[models.TagID]struct{}) error {
	if network == "" || len(tags) == 0 {
		return nil
	}
	eli, err := (&schema.Egress{Network: network}).ListByNetwork(db.WithContext(ctx))
	if err != nil {
		return err
	}
	for _, e := range eli {
		for tagID := range tags {
			if _, ok := e.Tags[tagID.String()]; ok {
				return ErrUserDeviceInfrastructureRole
			}
		}
	}
	return nil
}

func errIfEgressUsesTag(ctx context.Context, network string, tagID models.TagID) error {
	eli, err := (&schema.Egress{Network: network}).ListByNetwork(db.WithContext(ctx))
	if err != nil {
		return err
	}
	for _, e := range eli {
		if _, ok := e.Tags[tagID.String()]; ok {
			return ErrUserDeviceInfrastructureRole
		}
	}
	return nil
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
