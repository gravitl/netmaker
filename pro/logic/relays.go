package logic

import (
	"errors"
	"fmt"
	"net"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/acls/nodeacls"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
)

// GetRelays - gets all the nodes that are relays
func GetRelays() ([]models.Node, error) {
	nodes, err := logic.GetAllNodes()
	if err != nil {
		return nil, err
	}
	relays := make([]models.Node, 0)
	for _, node := range nodes {
		if node.IsRelay {
			relays = append(relays, node)
		}
	}
	return relays, nil
}

// CreateRelay - creates a relay
func CreateRelay(relay models.RelayRequest) ([]models.Node, models.Node, error) {
	var returnnodes []models.Node

	node, err := logic.GetNodeByID(relay.NodeID)
	if err != nil {
		return returnnodes, models.Node{}, err
	}
	host, err := logic.GetHost(node.HostID.String())
	if err != nil {
		return returnnodes, models.Node{}, err
	}
	if host.OS != "linux" {
		return returnnodes, models.Node{}, fmt.Errorf("only linux machines can be relay nodes")
	}
	err = ValidateRelay(relay)
	if err != nil {
		return returnnodes, models.Node{}, err
	}
	node.IsRelay = true
	node.RelayedNodes = relay.RelayedNodes
	node.SetLastModified()
	err = logic.UpsertNode(&node)
	if err != nil {
		return returnnodes, node, err
	}
	returnnodes = SetRelayedNodes(true, relay.NodeID, relay.RelayedNodes)
	return returnnodes, node, nil
}

// SetRelayedNodes- sets and saves node as relayed
func SetRelayedNodes(setRelayed bool, relay string, relayed []string) []models.Node {
	var returnnodes []models.Node
	for _, id := range relayed {
		node, err := logic.GetNodeByID(id)
		if err != nil {
			logger.Log(0, "setRelayedNodes.GetNodebyID", err.Error())
			continue
		}
		node.IsRelayed = setRelayed
		if setRelayed {
			node.RelayedBy = relay
		} else {
			node.RelayedBy = ""
		}
		node.SetLastModified()
		if err := logic.UpsertNode(&node); err != nil {
			logger.Log(0, "setRelayedNodes.Insert", err.Error())
			continue
		}
		returnnodes = append(returnnodes, node)
	}
	return returnnodes
}

// func GetRelayedNodes(relayNode *models.Node) (models.Node, error) {
//	var returnnodes []models.Node
//	networkNodes, err := GetNetworkNodes(relayNode.Network)
//	if err != nil {
//		return returnnodes, err
//	}
//	for _, node := range networkNodes {
//		for _, addr := range relayNode.RelayAddrs {
//			if addr == node.Address.IP.String() || addr == node.Address6.IP.String() {
//				returnnodes = append(returnnodes, node)
//			}
//		}
//	}
//	return returnnodes, nil
// }

// ValidateRelay - checks if relay is valid
func ValidateRelay(relay models.RelayRequest) error {
	var err error

	node, err := logic.GetNodeByID(relay.NodeID)
	if err != nil {
		return err
	}
	if node.IsRelay {
		return errors.New("node is already acting as a relay")
	}
	for _, relayedNodeID := range relay.RelayedNodes {
		relayedNode, err := logic.GetNodeByID(relayedNodeID)
		if err != nil {
			return err
		}
		if relayedNode.IsIngressGateway {
			return errors.New("cannot relay an ingress gateway (" + relayedNodeID + ")")
		}
	}
	return err
}

// UpdateRelayNodes - updates relay nodes
func updateRelayNodes(relay string, oldNodes []string, newNodes []string) []models.Node {
	_ = SetRelayedNodes(false, relay, oldNodes)
	return SetRelayedNodes(true, relay, newNodes)
}

func RelayUpdates(currentNode, newNode *models.Node) bool {
	relayUpdates := false
	if servercfg.IsPro && newNode.IsRelay && len(newNode.RelayedNodes) > 0 {
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
func UpdateRelayed(currentNode, newNode *models.Node) {
	updatenodes := updateRelayNodes(currentNode.ID.String(), currentNode.RelayedNodes, newNode.RelayedNodes)
	if len(updatenodes) > 0 {
		for _, relayedNode := range updatenodes {
			node := relayedNode
			ResetFailedOverPeer(&node)
			go func() {
				if err := mq.NodeUpdate(&node); err != nil {
					slog.Error("error publishing node update to node", "node", node.ID, "error", err)
				}

			}()
		}
	}
}

// DeleteRelay - deletes a relay
func DeleteRelay(network, nodeid string) ([]models.Node, models.Node, error) {
	var returnnodes []models.Node
	node, err := logic.GetNodeByID(nodeid)
	if err != nil {
		return returnnodes, models.Node{}, err
	}
	returnnodes = SetRelayedNodes(false, nodeid, node.RelayedNodes)
	node.IsRelay = false
	node.RelayedNodes = []string{}
	node.SetLastModified()
	if err = logic.UpsertNode(&node); err != nil {
		return returnnodes, models.Node{}, err
	}
	return returnnodes, node, nil
}

func RelayedAllowedIPs(peer, node *models.Node) []net.IPNet {
	var allowedIPs = []net.IPNet{}
	for _, relayedNodeID := range peer.RelayedNodes {
		if node.ID.String() == relayedNodeID {
			continue
		}
		relayedNode, err := logic.GetNodeByID(relayedNodeID)
		if err != nil {
			continue
		}
		allowed := getRelayedAddresses(relayedNodeID)
		if relayedNode.IsEgressGateway {
			allowed = append(allowed, logic.GetEgressIPs(&relayedNode)...)
		}
		allowedIPs = append(allowedIPs, allowed...)
	}
	return allowedIPs
}

// GetAllowedIpsForRelayed - returns the peerConfig for a node relayed by relay
func GetAllowedIpsForRelayed(relayed, relay *models.Node) (allowedIPs []net.IPNet) {
	if relayed.RelayedBy != relay.ID.String() {
		logger.Log(0, "RelayedByRelay called with invalid parameters")
		return
	}
	peers, err := logic.GetNetworkNodes(relay.Network)
	if err != nil {
		logger.Log(0, "error getting network clients", err.Error())
		return
	}
	for _, peer := range peers {
		if peer.ID == relayed.ID || peer.ID == relay.ID {
			continue
		}
		if nodeacls.AreNodesAllowed(nodeacls.NetworkID(relayed.Network), nodeacls.NodeID(relayed.ID.String()), nodeacls.NodeID(peer.ID.String())) {
			allowedIPs = append(allowedIPs, logic.GetAllowedIPs(relayed, &peer, nil)...)
		}
	}
	return
}

func getRelayedAddresses(id string) []net.IPNet {
	addrs := []net.IPNet{}
	node, err := logic.GetNodeByID(id)
	if err != nil {
		logger.Log(0, "getRelayedAddresses: "+err.Error())
		return addrs
	}
	if node.Address.IP != nil {
		node.Address.Mask = net.CIDRMask(32, 32)
		addrs = append(addrs, node.Address)
	}
	if node.Address6.IP != nil {
		node.Address.Mask = net.CIDRMask(128, 128)
		addrs = append(addrs, node.Address6)
	}
	return addrs
}
