package logic

import (
	"errors"
	"fmt"
	"net"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic/acls/nodeacls"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

// GetRelays - gets all the nodes that are relays
func GetRelays() ([]models.Node, error) {
	nodes, err := GetAllNodes()
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

	node, err := GetNodeByID(relay.NodeID)
	if err != nil {
		return returnnodes, models.Node{}, err
	}
	host, err := GetHost(node.HostID.String())
	if err != nil {
		return returnnodes, models.Node{}, err
	}
	if host.OS != "linux" {
		return returnnodes, models.Node{}, fmt.Errorf("only linux machines can be gateway nodes")
	}
	err = ValidateRelay(relay, false)
	if err != nil {
		return returnnodes, models.Node{}, err
	}
	node.IsRelay = true
	node.IsGw = true
	node.RelayedNodes = relay.RelayedNodes
	node.SetLastModified()
	err = UpsertNode(&node)
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
		}
		node.SetLastModified()
		if err := UpsertNode(&node); err != nil {
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
func ValidateRelay(relay models.RelayRequest, update bool) error {
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
		if relayedNode.IsInternetGateway {
			return errors.New("cannot relay an internet gateway (" + relayedNodeID + ")")
		}
		if relayedNode.InternetGwID != "" && relayedNode.InternetGwID != relay.NodeID {
			return errors.New("cannot relay an internet client (" + relayedNodeID + ")")
		}
		if relayedNode.IsFailOver {
			return errors.New("cannot relay a failOver (" + relayedNodeID + ")")
		}
		if relayedNode.FailedOverBy != uuid.Nil {
			ResetFailedOverPeer(&relayedNode)
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
	if servercfg.IsPro && newNode.IsRelay {
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

func RelayedAllowedIPs(peer, node *models.Node) []net.IPNet {
	var allowedIPs = []net.IPNet{}
	for _, relayedNodeID := range peer.RelayedNodes {
		if node.ID.String() == relayedNodeID {
			continue
		}
		relayedNode, err := GetNodeByID(relayedNodeID)
		if err != nil {
			continue
		}
		allowed := getRelayedAddresses(relayedNodeID)
		if relayedNode.IsEgressGateway {
			allowed = append(allowed, GetEgressIPs(&relayedNode)...)
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
	if relay.InternetGwID != "" {
		return GetAllowedIpForInetNodeClient(relayed, relay)
	}
	peers, err := GetNetworkNodes(relay.Network)
	if err != nil {
		logger.Log(0, "error getting network clients", err.Error())
		return
	}
	for _, peer := range peers {
		if peer.ID == relayed.ID || peer.ID == relay.ID {
			continue
		}
		if nodeacls.AreNodesAllowed(nodeacls.NetworkID(relayed.Network), nodeacls.NodeID(relayed.ID.String()), nodeacls.NodeID(peer.ID.String())) {
			allowedIPs = append(allowedIPs, GetAllowedIPs(relayed, &peer, nil)...)
		}
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
