package logic

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/gravitl/netmaker/database"
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

	err = ValidateRelay(relay)
	if err != nil {
		return returnnodes, models.Node{}, err
	}
	node.IsRelay = "yes"
	node.RelayAddrs = relay.RelayAddrs

	node.SetLastModified()
	node.PullChanges = "yes"
	nodeData, err := json.Marshal(&node)
	if err != nil {
		return returnnodes, node, err
	}
	if err = database.Insert(node.ID, string(nodeData), database.NODES_TABLE_NAME); err != nil {
		return returnnodes, models.Node{}, err
	}
	returnnodes, err = SetRelayedNodes("yes", node.Network, node.RelayAddrs)
	if err != nil {
		return returnnodes, node, err
	}
	if err = NetworkNodesUpdatePullChanges(node.Network); err != nil {
		return returnnodes, models.Node{}, err
	}
	return returnnodes, node, nil
}

// SetRelayedNodes- set relayed nodes
func SetRelayedNodes(yesOrno string, networkName string, addrs []string) ([]models.Node, error) {
	var returnnodes []models.Node
	collections, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		return returnnodes, err
	}
	network, err := GetNetworkSettings(networkName)
	if err != nil {
		return returnnodes, err
	}

	for _, value := range collections {

		var node models.Node
		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			return returnnodes, err
		}
		if node.Network == networkName && !(node.IsServer == "yes") {
			for _, addr := range addrs {
				if addr == node.Address || addr == node.Address6 {
					node.IsRelayed = yesOrno
					if yesOrno == "yes" {
						node.UDPHolePunch = "no"
					} else {
						node.UDPHolePunch = network.DefaultUDPHolePunch
					}
					data, err := json.Marshal(&node)
					if err != nil {
						return returnnodes, err
					}
					database.Insert(node.ID, string(data), database.NODES_TABLE_NAME)
					returnnodes = append(returnnodes, node)
				}
			}
		}
	}
	return returnnodes, nil
}

// SetNodeIsRelayed - Sets IsRelayed to on or off for relay
func SetNodeIsRelayed(yesOrno string, id string) error {
	node, err := GetNodeByID(id)
	if err != nil {
		return err
	}
	network, err := GetNetworkByNode(&node)
	if err != nil {
		return err
	}
	node.IsRelayed = yesOrno
	if yesOrno == "yes" {
		node.UDPHolePunch = "no"
	} else {
		node.UDPHolePunch = network.DefaultUDPHolePunch
	}
	data, err := json.Marshal(&node)
	if err != nil {
		return err
	}
	return database.Insert(node.ID, string(data), database.NODES_TABLE_NAME)
}

// PeerListUnRelay - call this function if a relayed node fails to get its relay: unrelays node and gets new peer list
func PeerListUnRelay(id string, network string) ([]models.Node, error) {
	err := SetNodeIsRelayed("no", id)
	if err != nil {
		return nil, err
	}
	return GetPeersList(network, true, "")
}

// ValidateRelay - checks if relay is valid
func ValidateRelay(relay models.RelayRequest) error {
	var err error
	//isIp := functions.IsIpCIDR(gateway.RangeString)
	empty := len(relay.RelayAddrs) == 0
	if empty {
		err = errors.New("IP Ranges Cannot Be Empty")
	}
	return err
}

// UpdateRelay - updates a relay
func UpdateRelay(network string, oldAddrs []string, newAddrs []string) []models.Node {
	var returnnodes []models.Node
	time.Sleep(time.Second / 4)
	returnnodes, err := SetRelayedNodes("no", network, oldAddrs)
	if err != nil {
		logger.Log(1, err.Error())
	}
	returnnodes, err = SetRelayedNodes("yes", network, newAddrs)
	if err != nil {
		logger.Log(1, err.Error())
	}
	return returnnodes
}

// DeleteRelay - deletes a relay
func DeleteRelay(network, nodeid string) ([]models.Node, models.Node, error) {
	var returnnodes []models.Node
	node, err := GetNodeByID(nodeid)
	if err != nil {
		return returnnodes, models.Node{}, err
	}
	_, err = SetRelayedNodes("no", node.Network, node.RelayAddrs)
	if err != nil {
		return returnnodes, node, err
	}

	node.IsRelay = "no"
	node.RelayAddrs = []string{}
	node.SetLastModified()
	node.PullChanges = "yes"

	data, err := json.Marshal(&node)
	if err != nil {
		return returnnodes, models.Node{}, err
	}
	if err = database.Insert(nodeid, string(data), database.NODES_TABLE_NAME); err != nil {
		return returnnodes, models.Node{}, err
	}
	if err = NetworkNodesUpdatePullChanges(network); err != nil {
		return returnnodes, models.Node{}, err
	}
	return returnnodes, node, nil
}
