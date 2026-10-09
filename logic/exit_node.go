package logic

import (
	"context"
	"errors"
	"net"
	"sort"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// ErrExitNodeSelectionRequired is returned when a user tries to clear the exit
// node on a network that requires one.
var ErrExitNodeSelectionRequired = errors.New("exit node selection is required")

// PublishPeerUpdateAfterExitNodeChange notifies peers after exit-node selection changes (wired from mq).
var PublishPeerUpdateAfterExitNodeChange = func(ctx context.Context) {}

func exitNodeItemFromEgress(ctx context.Context, e schema.Egress, selected bool) models.DeviceExitNode {
	routingNodeID := FirstInternetEgressRoutingNodeID(e)
	item := models.DeviceExitNode{
		EgressID:      e.ID,
		Name:          e.Name,
		Description:   e.Description,
		Network:       e.Network,
		RoutingNodeID: routingNodeID,
		Selected:      selected,
		Status:        e.Status,
	}
	if routingNodeID != "" {
		if rn, err := GetNodeByID(routingNodeID); err == nil {
			// Egress resource may still be enabled while the routing node is
			// disconnected — treat the exit as unavailable for selection/auto-pick.
			if !rn.Connected {
				item.Status = false
			}
			if rn.Address.IP != nil {
				item.Address = rn.Address.IP.String()
			}
			if rn.Address6.IP != nil {
				item.Address6 = rn.Address6.IP.String()
			}
			item.TcpProxyEnabled = rn.TcpProxyEnabled
			item.TcpProxyListenPort = rn.TcpProxyListenPort
			rh := &schema.Host{ID: rn.HostID}
			if err := rh.Get(db.WithContext(ctx)); err == nil {
				item.RoutingHostName = rh.Name
				item.CountryCode = rh.CountryCode
				item.Location = rh.Location
				item.AllowedEndpoints = exitNodeAllowedEndpoints(rh)
				if rh.TcpProxyEnabled {
					item.TcpProxyEnabled = true
				}
				if rh.TcpProxyListenPort > 0 {
					item.TcpProxyListenPort = rh.TcpProxyListenPort
				}
			}
			if item.TcpProxyEnabled && item.TcpProxyListenPort <= 0 {
				item.TcpProxyListenPort = schema.DefaultTcpProxyListenPort
			}
		} else {
			item.Status = false
		}
	} else if e.Status {
		item.Status = false
	}
	return item
}

// ListNodeExitNodes returns active internet egresses in the network for admin assignment.
func ListNodeExitNodes(ctx context.Context, network, nodeID string) ([]models.DeviceExitNode, error) {
	if network == "" || nodeID == "" {
		return nil, errors.New("network and node are required")
	}
	node, err := GetNodeByID(nodeID)
	if err != nil {
		return nil, errors.New("node not found")
	}
	if node.Network != network {
		return nil, errors.New("node not in network")
	}
	eli, err := (&schema.Egress{Network: network}).ListByNetwork(db.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]models.DeviceExitNode, 0)
	for _, e := range eli {
		if !e.Status || !IsEgressInternetGateway(e) {
			continue
		}
		out = append(out, exitNodeItemFromEgress(ctx, e, node.SelectedInternetEgressID == e.ID))
	}
	return out, nil
}

// GetNodeExitNode returns the internet egress currently assigned to the node.
func GetNodeExitNode(ctx context.Context, network, nodeID string) (*models.DeviceExitNode, error) {
	nodes, err := ListNodeExitNodes(ctx, network, nodeID)
	if err != nil {
		return nil, err
	}
	for i := range nodes {
		if nodes[i].Selected {
			return &nodes[i], nil
		}
	}
	return nil, nil
}

// AssignNodeExitNode sets or clears the internet egress for a node (admin; no ACL checks).
// useTcpUplink opts the client into TCP uplink to the exit routing gateway when that
// gateway has TCP proxy enabled; ignored when clearing the exit.
func AssignNodeExitNode(ctx context.Context, network, nodeID, egressID string, useTcpUplink bool) (*models.DeviceExitNode, error) {
	if network == "" || nodeID == "" {
		return nil, errors.New("network and node are required")
	}
	node, err := GetNodeByID(nodeID)
	if err != nil {
		return nil, errors.New("node not found")
	}
	if node.Network != network {
		return nil, errors.New("node not in network")
	}
	if egressID != "" {
		e := &schema.Egress{ID: egressID}
		if err := e.Get(db.WithContext(ctx)); err != nil {
			return nil, errors.New("exit node not found")
		}
		if !e.Status || e.Network != network || !IsEgressInternetGateway(*e) {
			return nil, errors.New("egress is not an active internet exit node in this network")
		}
	} else {
		useTcpUplink = false
	}
	if err := SetNodeSelectedInternetEgress(&node, egressID, useTcpUplink); err != nil {
		return nil, err
	}
	PublishPeerUpdateAfterExitNodeChange(ctx)
	if egressID == "" {
		return nil, nil
	}
	return GetNodeExitNode(ctx, network, nodeID)
}

// deviceMaySelectExitNode reports whether the device may list or select internet
// egress e. User-owned devices follow user policies only — All Resources must
// not expose every exit in the picker.
func deviceMaySelectExitNode(user *schema.User, host *schema.Host, node *models.Node, e *schema.Egress,
	deviceAcls, userAcls []models.Acl, defaultDeviceEnabled, defaultUserEnabled bool) bool {
	if user == nil || node == nil || e == nil {
		return false
	}
	if IsUserOwnedHost(host) || IsUserOwnedDevice(node) {
		if defaultUserEnabled {
			return true
		}
		return DoesUserHaveAccessToEgress(user, e, userAcls)
	}
	if defaultDeviceEnabled {
		return true
	}
	return DoesNodeHaveAccessToEgress(node, e, deviceAcls)
}

// ListDeviceExitNodes returns internet-type egresses in the network the user may select,
// filtered by ACL. User devices ignore the All Resources default; infra devices keep it.
func ListDeviceExitNodes(ctx context.Context, user *schema.User, host *schema.Host, networkID string) ([]models.DeviceExitNode, error) {
	if user == nil || host == nil {
		return nil, errors.New("user and host are required")
	}
	if networkID == "" {
		return nil, errors.New("network is required")
	}
	if !UserHasAccessToNetwork(ctx, user, networkID) {
		return nil, errors.New("user does not have access to network")
	}
	nodeSchema, err := getHostNodeOnNetwork(ctx, host, networkID)
	if err != nil {
		return nil, errors.New("device is not joined to network")
	}
	node := ConvertSchemaNodeToModelsNode(nodeSchema)
	if IsUserOwnedHost(host) && node.OwnerID == "" {
		node.OwnerID = host.OwnerUsername
	}

	eli, err := (&schema.Egress{Network: networkID}).ListByNetwork(ctx)
	if err != nil {
		return nil, err
	}
	acls := ListDevicePolicies(ctx, schema.NetworkID(networkID))
	userAcls := ListUserPolicies(ctx, schema.NetworkID(networkID))
	defaultDevicePolicy, _ := GetDefaultPolicy(ctx, schema.NetworkID(networkID), models.DevicePolicy)
	defaultUserPolicy, _ := GetDefaultPolicy(ctx, schema.NetworkID(networkID), models.UserPolicy)

	out := make([]models.DeviceExitNode, 0)
	for _, e := range eli {
		if !e.Status || !IsEgressInternetGateway(e) {
			continue
		}
		if !deviceMaySelectExitNode(user, host, node, &e, acls, userAcls,
			defaultDevicePolicy.Enabled, defaultUserPolicy.Enabled) {
			continue
		}
		out = append(out, exitNodeItemFromEgress(ctx, e, node.SelectedInternetEgressID == e.ID))
	}
	return out, nil
}

// GetDeviceSelectedExitNode returns the currently selected exit node for the device on the network.
func GetDeviceSelectedExitNode(ctx context.Context, user *schema.User, host *schema.Host, networkID string) (*models.DeviceExitNode, error) {
	nodes, err := ListDeviceExitNodes(ctx, user, host, networkID)
	if err != nil {
		return nil, err
	}
	for i := range nodes {
		if nodes[i].Selected {
			return &nodes[i], nil
		}
	}
	return nil, nil
}

// SelectDeviceExitNode sets or clears the selected internet egress for the device's node on the network.
// useTcpUplink opts into TCP uplink when the exit routing gateway has TCP proxy enabled.
// force allows clearing the exit even when the network requires auto_select_exit_node
// (used for clear-then-switch during auto failover).
func SelectDeviceExitNode(ctx context.Context, user *schema.User, host *schema.Host, networkID, egressID string, useTcpUplink, force bool) (*models.DeviceExitNode, error) {
	if user == nil || host == nil {
		return nil, errors.New("user and host are required")
	}
	if networkID == "" {
		return nil, errors.New("network is required")
	}
	if !UserHasAccessToNetwork(ctx, user, networkID) {
		return nil, errors.New("user does not have access to network")
	}
	if !UserHasDeviceNetworkWriteAccess(ctx, user, networkID) {
		return nil, errors.New("operation not permitted")
	}
	nodeSchema, err := getHostNodeOnNetwork(ctx, host, networkID)
	if err != nil {
		return nil, errors.New("device is not joined to network")
	}
	node := ConvertSchemaNodeToModelsNode(nodeSchema)
	if IsUserOwnedHost(host) && node.OwnerID == "" {
		node.OwnerID = host.OwnerUsername
	}

	if egressID != "" {
		e := &schema.Egress{ID: egressID}
		if err := e.Get(ctx); err != nil {
			return nil, errors.New("exit node not found")
		}
		if !e.Status || e.Network != networkID || !IsEgressInternetGateway(*e) {
			return nil, errors.New("egress is not an active internet exit node in this network")
		}
		acls := ListDevicePolicies(ctx, schema.NetworkID(networkID))
		userAcls := ListUserPolicies(ctx, schema.NetworkID(networkID))
		defaultDevicePolicy, _ := GetDefaultPolicy(ctx, schema.NetworkID(networkID), models.DevicePolicy)
		defaultUserPolicy, _ := GetDefaultPolicy(ctx, schema.NetworkID(networkID), models.UserPolicy)
		if !deviceMaySelectExitNode(user, host, node, e, acls, userAcls,
			defaultDevicePolicy.Enabled, defaultUserPolicy.Enabled) {
			return nil, errors.New("user does not have access to this exit node")
		}
		routingNodeID := FirstInternetEgressRoutingNodeID(*e)
		if routingNodeID == node.ID.String() {
			return nil, errors.New("routing node cannot select itself as exit node")
		}
	} else {
		nw := &schema.Network{Name: networkID}
		if err := nw.Get(ctx); err != nil {
			return nil, errors.New("network not found")
		}
		if nw.AutoSelectExitNode && !force {
			return nil, ErrExitNodeSelectionRequired
		}
		useTcpUplink = false
	}

	if err := SetNodeSelectedInternetEgress(node, egressID, useTcpUplink); err != nil {
		return nil, err
	}
	PublishPeerUpdateAfterExitNodeChange(ctx)

	if egressID == "" {
		return nil, nil
	}
	return GetDeviceSelectedExitNode(ctx, user, host, networkID)
}

// pickFallbackExitNode returns the exit to assign when auto-select is required
// and the node does not already have an allowed selection. The choice is the
// first allowed exit by name, then id. ok is false when the current selection
// should be kept or when nothing is available.
func pickFallbackExitNode(currentID string, nodes []models.DeviceExitNode) (models.DeviceExitNode, bool) {
	allowed := make([]models.DeviceExitNode, 0, len(nodes))
	for _, n := range nodes {
		if !n.Status || n.EgressID == "" {
			continue
		}
		allowed = append(allowed, n)
	}
	sort.Slice(allowed, func(i, j int) bool {
		if allowed[i].Name != allowed[j].Name {
			return allowed[i].Name < allowed[j].Name
		}
		return allowed[i].EgressID < allowed[j].EgressID
	})
	if currentID != "" {
		for _, n := range allowed {
			if n.EgressID == currentID {
				return models.DeviceExitNode{}, false
			}
		}
	}
	if len(allowed) == 0 {
		return models.DeviceExitNode{}, false
	}
	return allowed[0], true
}

// EnsureAutoExitNode assigns an allowed internet exit when a user device is on
// a network that requires one and none is selected. A still-valid selection is
// left unchanged so a client that already picked the nearest exit is not replaced.
func EnsureAutoExitNode(ctx context.Context, host *schema.Host, node *models.Node) error {
	if host == nil || node == nil || node.Network == "" {
		return nil
	}
	if !IsUserOwnedHost(host) && !IsUserOwnedDevice(node) {
		return nil
	}
	nw := &schema.Network{Name: node.Network}
	if err := nw.Get(ctx); err != nil {
		return err
	}
	if !nw.AutoSelectExitNode {
		return nil
	}
	username := host.OwnerUsername
	if username == "" {
		username = NodeOwnerUsername(node)
	}
	if username == "" {
		return nil
	}
	user := &schema.User{Username: username}
	if err := user.GetWithMembership(ctx); err != nil {
		return err
	}
	exits, err := ListDeviceExitNodes(ctx, user, host, node.Network)
	if err != nil {
		return err
	}
	pick, ok := pickFallbackExitNode(node.SelectedInternetEgressID, exits)
	if !ok {
		return nil
	}
	if err := SetNodeSelectedInternetEgress(node, pick.EgressID, false); err != nil {
		return err
	}
	PublishPeerUpdateAfterExitNodeChange(ctx)
	return nil
}

// exitNodeAllowedEndpoints returns the routing host public IPs (EndpointIP, EndpointIPv6).
func exitNodeAllowedEndpoints(host *schema.Host) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(ip net.IP) {
		if len(ip) == 0 || ip.IsUnspecified() || ip.IsLoopback() {
			return
		}
		s := ip.String()
		if s == "" || s == "<nil>" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	if host != nil {
		add(host.EndpointIP)
		add(host.EndpointIPv6)
	}
	return out
}
