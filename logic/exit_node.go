package logic

import (
	"context"
	"errors"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

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
			rh := &schema.Host{ID: rn.HostID}
			if err := rh.Get(db.WithContext(ctx)); err == nil {
				item.RoutingHostName = rh.Name
			}
		}
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
func AssignNodeExitNode(ctx context.Context, network, nodeID, egressID string) (*models.DeviceExitNode, error) {
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
	}
	if err := SetNodeSelectedInternetEgress(&node, egressID); err != nil {
		return nil, err
	}
	PublishPeerUpdateAfterExitNodeChange(ctx)
	if egressID == "" {
		return nil, nil
	}
	return GetNodeExitNode(ctx, network, nodeID)
}

// ListDeviceExitNodes returns internet-type egresses in the network the user may select,
// filtered by ACL when the default device policy is disabled.
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

	eli, err := (&schema.Egress{Network: networkID}).ListByNetwork(ctx)
	if err != nil {
		return nil, err
	}
	acls := ListDevicePolicies(ctx, schema.NetworkID(networkID))
	userAcls := ListUserPolicies(ctx, schema.NetworkID(networkID))
	defaultDevicePolicy, _ := GetDefaultPolicy(ctx, schema.NetworkID(networkID), models.DevicePolicy)
	defaultUserPolicy, _ := GetDefaultPolicy(ctx, schema.NetworkID(networkID), models.UserPolicy)
	allowAll := defaultDevicePolicy.Enabled || defaultUserPolicy.Enabled

	out := make([]models.DeviceExitNode, 0)
	for _, e := range eli {
		if !e.Status || !IsEgressInternetGateway(e) {
			continue
		}
		if !allowAll {
			if !DoesNodeHaveAccessToEgress(node, &e, acls) && !DoesUserHaveAccessToEgress(user, &e, userAcls) {
				continue
			}
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
func SelectDeviceExitNode(ctx context.Context, user *schema.User, host *schema.Host, networkID, egressID string) (*models.DeviceExitNode, error) {
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
		allowAll := defaultDevicePolicy.Enabled || defaultUserPolicy.Enabled
		if !allowAll && !DoesNodeHaveAccessToEgress(node, e, acls) && !DoesUserHaveAccessToEgress(user, e, userAcls) {
			return nil, errors.New("user does not have access to this exit node")
		}
		routingNodeID := FirstInternetEgressRoutingNodeID(*e)
		if routingNodeID == node.ID.String() {
			return nil, errors.New("routing node cannot select itself as exit node")
		}
	}

	if err := SetNodeSelectedInternetEgress(node, egressID); err != nil {
		return nil, err
	}
	PublishPeerUpdateAfterExitNodeChange(ctx)

	if egressID == "" {
		return nil, nil
	}
	return GetDeviceSelectedExitNode(ctx, user, host, networkID)
}
