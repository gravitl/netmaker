package logic

import (
	"context"
	"errors"
	"net"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// SetRelayedNodes- sets and saves node as relayed
func SetRelayedNodes(setRelayed bool, relay string, relayed []string) []models.Node {
	var returnnodes []models.Node
	for _, id := range relayed {
		node, err := GetNodeByID(id)
		if err != nil {
			logger.Log(0, "setRelayedNodes.GetNodebyID", err.Error())
			continue
		}
		node.IsRelayed = setRelayed
		if setRelayed {
			node.RelayedBy = relay
		} else {
			node.RelayedBy = ""
			node.UseTcpUplink = false
		}
		node.SetLastModified()
		if err := UpsertNode(&node); err != nil {
			logger.Log(0, "setRelayedNodes.Insert", err.Error())
			continue
		}
		returnnodes = append(returnnodes, node)
	}
	relayNode, _ := GetNodeByID(relay)
	if setRelayed {
		relayNode.RelayedNodes = relayed
	} else {
		relayNode.RelayedNodes = []string{}
	}
	UpsertNode(&relayNode)
	return returnnodes
}

// ValidateRelay - checks if relay is valid
func ValidateRelay(ctx context.Context, relay models.RelayRequest, update bool) error {
	var err error

	node, err := GetNodeByID(relay.NodeID)
	if err != nil {
		return err
	}
	if !update && node.IsRelay {
		return errors.New("node is already acting as a relay")
	}
	for _, relayedNodeID := range relay.RelayedNodes {
		relayedNode, err := GetNodeByID(relayedNodeID)
		if err != nil {
			return err
		}
		if relayedNode.IsIngressGateway {
			return errors.New("cannot relay an ingress gateway (" + relayedNodeID + ")")
		}
		if relayedNode.IsInternetGateway || NodeIsInternetEgressRouter(relayedNode.ID.String(), relayedNode.Network) {
			return errors.New("cannot relay an internet gateway (" + relayedNodeID + ")")
		}
		// Exit clients are relayed only via exit-node selection (background AssignGateway).
		// Allow them when they are already exit-relayed by THIS node (idempotent gateway
		// / reconnect updates). Reject only when trying to manually attach them here.
		if relayedNode.SelectedInternetEgressID != "" {
			exitID := InternetExitRoutingNodeID(&relayedNode)
			if relayedNode.RelayedBy != relay.NodeID && exitID != relay.NodeID {
				return errors.New("cannot manually relay an exit node client (" + relayedNodeID + ")")
			}
			continue
		}
		if relayedNode.IsAutoRelay {
			return errors.New("cannot relay a auto relay node (" + relayedNodeID + ")")
		}
		if len(relayedNode.AutoRelayedPeers) > 0 {
			ResetAutoRelayedPeer(ctx, &relayedNode)
		}
	}
	return err
}

// UpdateRelayNodes - updates relay nodes
func UpdateRelayNodes(relay string, oldNodes []string, newNodes []string) []models.Node {
	_ = SetRelayedNodes(false, relay, oldNodes)
	return SetRelayedNodes(true, relay, newNodes)
}

func RelayUpdates(currentNode, newNode *models.Node) bool {
	relayUpdates := false
	if newNode.IsRelay {
		if len(newNode.RelayedNodes) != len(currentNode.RelayedNodes) {
			relayUpdates = true
		} else {
			for i, node := range newNode.RelayedNodes {
				if node != currentNode.RelayedNodes[i] {
					relayUpdates = true
				}
			}
		}
	}
	return relayUpdates
}

// UpdateRelayed - updates a relay's relayed nodes, and sends updates to the relayed nodes over MQ
func UpdateRelayed(ctx context.Context, currentNode, newNode *models.Node) {
	updatenodes := UpdateRelayNodes(currentNode.ID.String(), currentNode.RelayedNodes, newNode.RelayedNodes)
	if len(updatenodes) > 0 {
		for _, relayedNode := range updatenodes {
			node := relayedNode
			ResetAutoRelayedPeer(ctx, &node)
		}
	}
}

// DeleteRelay - deletes a relay
func DeleteRelay(network, nodeid string) ([]models.Node, models.Node, error) {
	var returnnodes []models.Node
	node, err := GetNodeByID(nodeid)
	if err != nil {
		return returnnodes, models.Node{}, err
	}
	returnnodes = SetRelayedNodes(false, nodeid, node.RelayedNodes)
	node.IsRelay = false
	node.RelayedNodes = []string{}
	node.SetLastModified()
	if err = UpsertNode(&node); err != nil {
		return returnnodes, models.Node{}, err
	}
	return returnnodes, node, nil
}

func RelayedAllowedIPs(ctx context.Context, peer, node *models.Node) []net.IPNet {
	var allowedIPs = []net.IPNet{}
	eli, _ := (&schema.Egress{Network: node.Network}).ListByNetwork(ctx)
	acls, _ := ListAclsByNetwork(ctx, schema.NetworkID(node.Network))
	for _, relayedNodeID := range peer.RelayedNodes {
		if node.ID.String() == relayedNodeID {
			continue
		}
		relayedNode, err := GetNodeByID(relayedNodeID)
		if err != nil {
			continue
		}
		GetNodeEgressInfo(&relayedNode, eli, acls)
		allowed := getRelayedAddresses(relayedNodeID)
		if relayedNode.EgressDetails.IsEgressGateway {
			allowed = append(allowed, GetEgressIPs(&relayedNode)...)
		}
		allowedIPs = append(allowedIPs, allowed...)
	}
	return allowedIPs
}

// GetAllowedIpsForRelayed - returns the peerConfig for a node relayed by relay
func GetAllowedIpsForRelayed(ctx context.Context, relayed, relay *models.Node) (allowedIPs []net.IPNet) {
	if relayed.RelayedBy != relay.ID.String() {
		logger.Log(0, "RelayedByRelay called with invalid parameters")
		return
	}
	peers, err := GetNetworkNodes(ctx, relay.Network)
	if err != nil {
		logger.Log(0, "error getting network clients", err.Error())
		return
	}
	acls, _ := ListAclsByNetwork(ctx, schema.NetworkID(relay.Network))
	eli, _ := (&schema.Egress{Network: relay.Network}).ListByNetwork(ctx)
	defaultPolicy, _ := GetDefaultPolicy(ctx, schema.NetworkID(relay.Network), models.DevicePolicy)
	for _, peer := range peers {
		if peer.ID == relayed.ID || peer.ID == relay.ID {
			continue
		}
		if !IsPeerAllowed(ctx, *relayed, peer, true) {
			continue
		}
		AddEgressInfoToPeerByAccess(relayed, &peer, eli, acls, defaultPolicy.Enabled)
		allowedIPs = append(allowedIPs, GetAllowedIPs(ctx, relayed, &peer, nil)...)
	}
	return
}

func getRelayedAddresses(id string) []net.IPNet {
	addrs := []net.IPNet{}
	node, err := GetNodeByID(id)
	if err != nil {
		logger.Log(0, "getRelayedAddresses: "+err.Error())
		return addrs
	}
	if node.Address.IP != nil {
		node.Address.Mask = net.CIDRMask(32, 32)
		addrs = append(addrs, node.Address)
	}
	if node.Address6.IP != nil {
		node.Address6.Mask = net.CIDRMask(128, 128)
		addrs = append(addrs, node.Address6)
	}
	return addrs
}

// ExitClientOverlayIPs returns /32 and /128 overlay addresses for clients in
// peer.RelayedNodes and peer.InetNodeReq.InetNodeClientIDs, excluding excludeID.
func ExitClientOverlayIPs(peer *models.Node, excludeID string) []net.IPNet {
	if peer == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []net.IPNet
	add := func(id string) {
		if id == "" || id == excludeID {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, getRelayedAddresses(id)...)
	}
	for _, id := range peer.RelayedNodes {
		add(id)
	}
	for _, id := range peer.InetNodeReq.InetNodeClientIDs {
		add(id)
	}
	return out
}

// ExitClientOverlayIPsFromInetClients returns overlay IPs for InetNodeClientIDs
// that are not already listed in RelayedNodes (RelayedAllowedIPs covers those).
func ExitClientOverlayIPsFromInetClients(peer *models.Node, excludeID string) []net.IPNet {
	if peer == nil || len(peer.InetNodeReq.InetNodeClientIDs) == 0 {
		return nil
	}
	inRelayed := map[string]struct{}{}
	for _, id := range peer.RelayedNodes {
		inRelayed[id] = struct{}{}
	}
	var out []net.IPNet
	for _, id := range peer.InetNodeReq.InetNodeClientIDs {
		if id == "" || id == excludeID {
			continue
		}
		if _, ok := inRelayed[id]; ok {
			continue
		}
		out = append(out, getRelayedAddresses(id)...)
	}
	return out
}
