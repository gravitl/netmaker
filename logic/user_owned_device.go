package logic

import (
	"context"
	"errors"

	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"golang.org/x/exp/slog"
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

// ListPostureViolatedUserOwnedDevices returns host-backed user devices on the
// network that have a non-unknown posture severity (for Non-compliant Users).
func ListPostureViolatedUserOwnedDevices(ctx context.Context, networkName string) ([]models.Node, error) {
	network := &schema.Network{Name: networkName}
	if err := network.Get(ctx); err != nil {
		return nil, err
	}
	_nodes, err := (&schema.Node{}).ListAll(
		ctx,
		dbtypes.WithPreloads("Host"),
		dbtypes.WithFilter("network_id", network.ID),
		dbtypes.WithNotFilter("posture_check_severity", schema.SeverityUnknown),
	)
	if err != nil {
		return nil, err
	}
	out := make([]models.Node, 0)
	for i := range _nodes {
		_nodes[i].Network = network
		node := ConvertSchemaNodeToModelsNodeWithContext(ctx, &_nodes[i])
		if node == nil || !IsUserOwnedDevice(node) {
			continue
		}
		out = append(out, *node)
	}
	return out, nil
}

// skipPeerUpdateAclCalc reports whether GetPeerUpdateForHost should skip
// FwUpdate ACL calculation. User policies are unidirectional to servers and
// are emitted on the server host's peer update instead.
func skipPeerUpdateAclCalc(h *schema.Host) bool {
	return IsUserOwnedHost(h)
}

// userDeviceEgressDefaultActive reports whether the network's default user
// policy grants all egress to user devices. Device default must not.
var userDeviceEgressDefaultActive = func(ctx context.Context, network string) bool {
	userDefault, err := GetDefaultPolicy(ctx, schema.NetworkID(network), models.UserPolicy)
	return err == nil && userDefault.Enabled
}

// UserDevicesAreNotPeers reports whether two hosts must not form a WireGuard peer.
// User-registered devices mesh with infrastructure, not with other user devices.
func UserDevicesAreNotPeers(a, b *schema.Host) bool {
	if a == nil || b == nil || a.ID == b.ID {
		return false
	}
	return IsUserOwnedHost(a) && IsUserOwnedHost(b)
}

// nodeFlowIdentity returns the flow-log identity for a node's addresses.
// User-registered devices are attributed to their owner, matching how
// Remote Access Client ExtClients are reported.
func nodeFlowIdentity(node *models.Node, host *schema.Host) models.PeerIdentity {
	if IsUserOwnedHost(host) {
		return models.PeerIdentity{
			ID:   host.OwnerUsername,
			Type: models.PeerType_User,
			Name: host.OwnerUsername,
		}
	}
	return models.PeerIdentity{
		ID:   node.ID.String(),
		Type: models.PeerType_Node,
		Name: host.Name,
	}
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

// skipResourcePolicyForUserDevices reports whether resource (device) ACL
// policies must not decide this pair. User policies are the access control.
func skipResourcePolicyForUserDevices(a, b *models.Node) bool {
	return IsUserOwnedDevice(a) || IsUserOwnedDevice(b)
}

// PeerAllowed reports whether node may peer with peer. User devices are not
// subject to resource ACL policies, including the default allow-all.
func PeerAllowed(ctx context.Context, node, peer models.Node, defaultDevicePolicy bool) bool {
	if skipResourcePolicyForUserDevices(&node, &peer) {
		return isAllowedViaUserOwnership(ctx, node, peer)
	}
	if defaultDevicePolicy {
		return true
	}
	return IsPeerAllowed(ctx, node, peer, false) || isAllowedViaUserOwnership(ctx, node, peer)
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
	// Resource policies do not grant egress to user devices.
	if !IsUserOwnedDevice(node) && DoesNodeHaveAccessToEgress(node, e, deviceAcls) {
		return true
	}
	owner := NodeOwnerUsername(node)
	if owner == "" {
		return false
	}
	if userDeviceEgressDefaultActive(ctx, node.Network) {
		return true
	}
	user := &schema.User{Username: owner}
	if err := user.GetWithMembership(ctx); err != nil {
		return false
	}
	return DoesUserHaveAccessToEgress(user, e, ListUserPolicies(ctx, schema.NetworkID(node.Network)))
}

// UserDeviceNode is a network node of a user-registered device.
type UserDeviceNode struct {
	Node models.Node
	Host schema.Host
}

// UserDevice is a deleted user-registered host along with its deleted nodes.
type UserDevice struct {
	Host  schema.Host
	Nodes []models.Node
}

func listUserDeviceNodes(ctx context.Context, username string) (map[string][]models.Node, error) {
	_nodes, err := (&schema.Node{}).ListByOwnerUsername(ctx, username, dbtypes.WithAllPreloads())
	if err != nil {
		return nil, err
	}
	byHost := make(map[string][]models.Node)
	for i := range _nodes {
		node := ConvertSchemaNodeToModelsNode(&_nodes[i])
		byHost[_nodes[i].HostID] = append(byHost[_nodes[i].HostID], *node)
	}
	return byHost, nil
}

// DeleteUserDevices deletes the hosts registered by username, along with their
// nodes. It returns the deleted hosts so callers can publish them.
func DeleteUserDevices(ctx context.Context, username string) []UserDevice {
	hosts, err := (&schema.Host{}).ListAll(ctx, dbtypes.WithFilter("owner_username", username))
	if err != nil {
		slog.Error("failed to list user devices", "user", username, "error", err)
		return nil
	}
	// Without the node list, RemoveHost would delete nodes that peers are never told about.
	nodesByHost, err := listUserDeviceNodes(ctx, username)
	if err != nil {
		slog.Error("failed to list user device nodes", "user", username, "error", err)
		return nil
	}
	var deleted []UserDevice
	for i := range hosts {
		host := &hosts[i]
		device := UserDevice{Host: *host, Nodes: nodesByHost[host.ID.String()]}
		if err := RemoveHost(ctx, host, true); err != nil {
			slog.Error("failed to delete user device", "host", host.ID.String(), "user", username, "error", err)
			continue
		}
		_ = (&schema.PendingHost{HostID: host.ID.String()}).DeleteAllPendingHosts(ctx)
		deleted = append(deleted, device)
	}
	return deleted
}

// DisconnectUserDeviceNodes sets Connected=false on the user's device nodes in
// networks matching shouldDisconnect. Nodes stay enrolled so a later JIT grant
// can reconnect without re-approval. Returns affected nodes for peer publish.
func DisconnectUserDeviceNodes(ctx context.Context, username string, shouldDisconnect func(network string) bool) []UserDeviceNode {
	hosts, err := (&schema.Host{}).ListAll(ctx, dbtypes.WithFilter("owner_username", username))
	if err != nil {
		slog.Error("failed to list user devices", "user", username, "error", err)
		return nil
	}
	nodesByHost, err := listUserDeviceNodes(ctx, username)
	if err != nil {
		slog.Error("failed to list user device nodes", "user", username, "error", err)
		return nil
	}
	var disconnected []UserDeviceNode
	for _, host := range hosts {
		for _, node := range nodesByHost[host.ID.String()] {
			if !shouldDisconnect(node.Network) {
				continue
			}
			if !node.Connected {
				continue
			}
			if _, err := ClearExitNodeForDisconnect(&node); err != nil {
				slog.Error("failed to clear exit node on JIT disconnect",
					"node", node.ID.String(), "user", username, "network", node.Network, "error", err)
			}
			node.Connected = false
			node.Status = schema.Disconnected
			if err := UpsertNode(&node); err != nil {
				slog.Error("failed to disconnect user device node",
					"node", node.ID.String(), "user", username, "network", node.Network, "error", err)
				continue
			}
			disconnected = append(disconnected, UserDeviceNode{Node: node, Host: host})
		}
	}
	return disconnected
}

// DeleteUserDeviceNodes deletes the network nodes of devices registered by
// username in networks for which shouldDelete returns true. It returns the
// deleted nodes so callers can publish them.
func DeleteUserDeviceNodes(ctx context.Context, username string, shouldDelete func(network string) bool) []UserDeviceNode {
	hosts, err := (&schema.Host{}).ListAll(ctx, dbtypes.WithFilter("owner_username", username))
	if err != nil {
		slog.Error("failed to list user devices", "user", username, "error", err)
		return nil
	}
	nodesByHost, err := listUserDeviceNodes(ctx, username)
	if err != nil {
		slog.Error("failed to list user device nodes", "user", username, "error", err)
		return nil
	}
	var deleted []UserDeviceNode
	for _, host := range hosts {
		for _, node := range nodesByHost[host.ID.String()] {
			if !shouldDelete(node.Network) {
				continue
			}
			if err := DeleteNode(ctx, &node, true); err != nil {
				slog.Error("failed to delete user device node",
					"node", node.ID.String(), "user", username, "network", node.Network, "error", err)
				continue
			}
			deleted = append(deleted, UserDeviceNode{Node: node, Host: host})
		}
	}
	return deleted
}

// DeleteUserDeviceNodesWithoutAccess deletes the user's device nodes in
// networks the user can no longer access.
func DeleteUserDeviceNodesWithoutAccess(ctx context.Context, user *schema.User) []UserDeviceNode {
	return DeleteUserDeviceNodes(ctx, user.Username, func(network string) bool {
		return !UserHasAccessToNetwork(ctx, user, network)
	})
}
