package logic

import (
	"context"
	"errors"
	"net"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"golang.org/x/exp/slog"
	"gorm.io/gorm"
)

// SetRelayedNodes- sets and saves node as relayed
func SetRelayedNodes(setRelayed bool, relay string, relayed []string) []models.Node {
	var returnnodes []models.Node
	validRelayed := make([]string, 0, len(relayed))
	for _, id := range relayed {
		node, err := GetNodeByID(id)
		if err != nil {
			logger.Log(0, "setRelayedNodes.GetNodebyID", err.Error())
			continue
		}
		node.IsRelayed = setRelayed
		if setRelayed {
			node.RelayedBy = relay
			validRelayed = append(validRelayed, id)
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
	relayNode, err := GetNodeByID(relay)
	if err != nil {
		return returnnodes
	}
	if setRelayed {
		relayNode.RelayedNodes = validRelayed
	} else {
		relayNode.RelayedNodes = []string{}
	}
	_ = UpsertNode(&relayNode)
	return returnnodes
}

// ValidateRelay - checks if relay is valid
func ValidateRelay(ctx context.Context, relay models.RelayRequest, update bool) error {
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
			// Stale RelayedClients keys (deleted/unassigned nodes) must not block
			// gateway updates — skip and let PruneStaleRelayedClients clean them.
			if errors.Is(err, gorm.ErrRecordNotFound) {
				slog.Warn("ValidateRelay: skipping missing relayed node", "id", relayedNodeID, "relay", relay.NodeID)
				continue
			}
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
	return nil
}

// isActiveRelayedClient reports whether client is still assigned to gatewayID.
func isActiveRelayedClient(gatewayID string, client *models.Node) bool {
	if client == nil || gatewayID == "" {
		return false
	}
	if client.RelayedBy == gatewayID {
		return true
	}
	// Exit clients may still be relayed via the exit routing node even when
	// RelayedBy was briefly cleared; keep them if this gateway is their exit router.
	return InternetExitRoutingNodeID(client) == gatewayID
}

// PruneStaleRelayedClients drops RelayedNodes / InetNodeReq entries that no longer
// belong on this gateway (missing nodes, or nodes no longer RelayedBy it) and
// persists when anything changed. Returns true if pruned.
func PruneStaleRelayedClients(node *models.Node) bool {
	if node == nil || (!node.IsGw && !node.IsRelay && len(node.RelayedNodes) == 0 && len(node.InetNodeReq.InetNodeClientIDs) == 0) {
		return false
	}
	gatewayID := node.ID.String()
	changed := false
	alive := make([]string, 0, len(node.RelayedNodes))
	for _, id := range node.RelayedNodes {
		client, err := GetNodeByID(id)
		if err != nil || !isActiveRelayedClient(gatewayID, &client) {
			slog.Warn("pruning stale relayed_clients entry", "gateway", gatewayID, "stale", id)
			changed = true
			continue
		}
		alive = append(alive, id)
	}
	if changed {
		node.RelayedNodes = alive
	}

	if len(node.InetNodeReq.InetNodeClientIDs) > 0 {
		aliveIGW := make([]string, 0, len(node.InetNodeReq.InetNodeClientIDs))
		for _, id := range node.InetNodeReq.InetNodeClientIDs {
			client, err := GetNodeByID(id)
			if err != nil || !isActiveRelayedClient(gatewayID, &client) {
				slog.Warn("pruning stale relayed_igw_clients entry", "gateway", gatewayID, "stale", id)
				changed = true
				continue
			}
			aliveIGW = append(aliveIGW, id)
		}
		node.InetNodeReq.InetNodeClientIDs = aliveIGW
	}

	if !changed {
		return false
	}
	if err := UpsertNode(node); err != nil {
		slog.Error("failed to persist pruned relayed clients", "gateway", node.ID, "error", err)
		return false
	}
	return true
}

// SanitizeRelayedNodesForUpdate drops missing / orphan RelayedNodes IDs from a
// gateway update payload while still allowing genuinely new assignments
// (node exists, RelayedBy empty, and ID was not already a stale map key).
func SanitizeRelayedNodesForUpdate(gatewayID string, requested []string, priorMapKeys map[string]struct{}) []string {
	out := make([]string, 0, len(requested))
	for _, id := range requested {
		client, err := GetNodeByID(id)
		if err != nil {
			slog.Warn("dropping missing relayed node from update", "gateway", gatewayID, "id", id)
			continue
		}
		if client.RelayedBy == gatewayID || InternetExitRoutingNodeID(&client) == gatewayID {
			out = append(out, id)
			continue
		}
		if client.RelayedBy == "" {
			if _, wasStaleKey := priorMapKeys[id]; wasStaleKey {
				// Orphan map key: client was removed but RelayedClients still listed it.
				continue
			}
			out = append(out, id)
			continue
		}
		// Relayed by a different gateway — do not steal via this update.
	}
	return out
}

// RemoveNodeFromAllGatewayRelays removes nodeID from RelayedClients / RelayedIGWClients
// on every node in the network. Used on delete so orphan map keys cannot linger.
func RemoveNodeFromAllGatewayRelays(ctx context.Context, networkName, nodeID string) {
	if networkName == "" || nodeID == "" {
		return
	}
	nw := &schema.Network{Name: networkName}
	if err := nw.Get(ctx); err != nil {
		return
	}
	n := &schema.Node{ID: nodeID, NetworkID: nw.ID}
	if err := n.UnassignGateway(ctx); err != nil {
		slog.Error("failed to remove node from gateway relay maps", "node", nodeID, "error", err)
	}
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
