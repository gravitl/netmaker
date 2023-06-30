package logic

import (
	"errors"
	"fmt"
	"net"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
)

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
		return returnnodes, models.Node{}, fmt.Errorf("only linux machines can be relay nodes")
	}
	err = ValidateRelay(relay)
	if err != nil {
		return returnnodes, models.Node{}, err
	}
	node.IsRelay = true
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
		if node.IsRelayed {
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

//func GetRelayedNodes(relayNode *models.Node) (models.Node, error) {
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
//}

// ValidateRelay - checks if relay is valid
func ValidateRelay(relay models.RelayRequest) error {
	var err error
	//isIp := functions.IsIpCIDR(gateway.RangeString)
	empty := len(relay.RelayedNodes) == 0
	if empty {
		return errors.New("IP Ranges Cannot Be Empty")
	}
	node, err := GetNodeByID(relay.NodeID)
	if err != nil {
		return err
	}
	if node.IsRelay {
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
	}
	return err
}

// UpdateRelayed - updates relay nodes
func UpdateRelayed(relay string, oldNodes []string, newNodes []string) []models.Node {
	_ = SetRelayedNodes(false, relay, oldNodes)
	return SetRelayedNodes(true, relay, newNodes)
}

// GetDeletedRelayedNode - fetches deleted relayed node id
func GetDeletedRelayedNode(currRelay, updatedRelay models.Node) (deletedRelayedNodeID string) {
	updatedRelayedNodeMap := make(map[string]struct{})
	for _, relayedNodeID := range updatedRelay.RelayedNodes {
		updatedRelayedNodeMap[relayedNodeID] = struct{}{}
	}

	for _, relayedNodeID := range currRelay.RelayedNodes {
		if _, ok := updatedRelayedNodeMap[relayedNodeID]; !ok {
			deletedRelayedNodeID = relayedNodeID
			break
		}
	}
	return
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
		node.Address.Mask = net.CIDRMask(128, 128)
		addrs = append(addrs, node.Address6)
	}
	return addrs
}
