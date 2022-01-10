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
func CreateRelay(relay models.RelayRequest) (models.Node, error) {
	node, err := GetNodeByMacAddress(relay.NetID, relay.NodeID)
	if node.OS == "macos" { // add in darwin later
		return models.Node{}, errors.New(node.OS + " is unsupported for relay")
	}
	if err != nil {
		return models.Node{}, err
	}
	err = ValidateRelay(relay)
	if err != nil {
		return models.Node{}, err
	}
	node.IsRelay = "yes"
	node.RelayAddrs = relay.RelayAddrs

	key, err := GetRecordKey(relay.NodeID, relay.NetID)
	if err != nil {
		return node, err
	}
	node.SetLastModified()
	node.PullChanges = "yes"
	nodeData, err := json.Marshal(&node)
	if err != nil {
		return node, err
	}
	if err = database.Insert(key, string(nodeData), database.NODES_TABLE_NAME); err != nil {
		return models.Node{}, err
	}
	err = SetRelayedNodes("yes", node.Network, node.RelayAddrs)
	if err != nil {
		return node, err
	}

	if err = NetworkNodesUpdatePullChanges(node.Network); err != nil {
		return models.Node{}, err
	}
	return node, nil
}

// SetRelayedNodes- set relayed nodes
func SetRelayedNodes(yesOrno string, networkName string, addrs []string) error {

	collections, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		return err
	}

	for _, value := range collections {

		var node models.Node
		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			return err
		}
		if node.Network == networkName {
			for _, addr := range addrs {
				if addr == node.Address || addr == node.Address6 {
					node.IsRelayed = yesOrno
					data, err := json.Marshal(&node)
					if err != nil {
						return err
					}
					database.Insert(node.ID, string(data), database.NODES_TABLE_NAME)
				}
			}
		}
	}
	return nil
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
func UpdateRelay(network string, oldAddrs []string, newAddrs []string) {
	time.Sleep(time.Second / 4)
	err := SetRelayedNodes("no", network, oldAddrs)
	if err != nil {
		logger.Log(1, err.Error())
	}
	err = SetRelayedNodes("yes", network, newAddrs)
	if err != nil {
		logger.Log(1, err.Error())
	}
}

// DeleteRelay - deletes a relay
func DeleteRelay(network, macaddress string) (models.Node, error) {

	node, err := GetNodeByMacAddress(network, macaddress)
	if err != nil {
		return models.Node{}, err
	}
	err = SetRelayedNodes("no", node.Network, node.RelayAddrs)
	if err != nil {
		return node, err
	}

	node.IsRelay = "no"
	node.RelayAddrs = []string{}
	node.SetLastModified()
	node.PullChanges = "yes"
	key, err := GetRecordKey(node.MacAddress, node.Network)
	if err != nil {
		return models.Node{}, err
	}
	data, err := json.Marshal(&node)
	if err != nil {
		return models.Node{}, err
	}
	if err = database.Insert(key, string(data), database.NODES_TABLE_NAME); err != nil {
		return models.Node{}, err
	}
	if err = NetworkNodesUpdatePullChanges(network); err != nil {
		return models.Node{}, err
	}
	return node, nil
}
