package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
)

// CreateRelay - creates a relay
func CreateRelay(relay models.RelayRequest) ([]models.Client, models.Node, error) {
	var relayedClients []models.Client
	node, err := GetNodeByID(relay.NodeID)
	if err != nil {
		return relayedClients, models.Node{}, err
	}
	host, err := GetHost(node.HostID.String())
	if err != nil {
		return relayedClients, models.Node{}, err
	}
	if host.OS != "linux" {
		return relayedClients, models.Node{}, fmt.Errorf("only linux machines can be relay nodes")
	}
	err = ValidateRelay(relay)
	if err != nil {
		return relayedClients, models.Node{}, err
	}
	node.IsRelay = true
	node.RelayedNodes = relay.RelayedNodes
	node.SetLastModified()
	nodeData, err := json.Marshal(&node)
	if err != nil {
		return relayedClients, node, err
	}
	if err = database.Insert(node.ID.String(), string(nodeData), database.NODES_TABLE_NAME); err != nil {
		return relayedClients, models.Node{}, err
	}
	relayedClients = SetRelayedNodes(true, relay.NodeID, relay.RelayedNodes)
	for _, relayed := range relayedClients {
		data, err := json.Marshal(&relayed.Node)
		if err != nil {
			logger.Log(0, "marshalling relayed node", err.Error())
			continue
		}
		if err := database.Insert(relayed.Node.ID.String(), string(data), database.NODES_TABLE_NAME); err != nil {
			logger.Log(0, "inserting relayed node", err.Error())
			continue
		}
	}
	return relayedClients, node, nil
}

// SetRelayedNodes- sets and saves node as relayed
func SetRelayedNodes(setRelayed bool, relay string, relayed []string) []models.Client {
	var returnnodes []models.Client
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
		data, err := json.Marshal(&node)
		if err != nil {
			logger.Log(0, "setRelayedNodes.Marshal", err.Error())
			continue
		}
		if err := database.Insert(node.ID.String(), string(data), database.NODES_TABLE_NAME); err != nil {
			logger.Log(0, "setRelayedNodes.Insert", err.Error())
			continue
		}
		host := GetHostByNodeID(node.ID.String())
		returnnodes = append(returnnodes, models.Client{
			Host: *host,
			Node: node,
		})
	}
	return returnnodes
}

// ValidateRelay - checks if relay is valid
func ValidateRelay(relay models.RelayRequest) error {
	var err error
	//isIp := functions.IsIpCIDR(gateway.RangeString)
	empty := len(relay.RelayedNodes) == 0
	if empty {
		err = errors.New("IP Ranges Cannot Be Empty")
	}
	return err
}

// UpdateRelayed - updates relay nodes
func UpdateRelayed(relay string, oldNodes []string, newNodes []string) []models.Client {
	_ = SetRelayedNodes(false, relay, oldNodes)
	return SetRelayedNodes(true, relay, newNodes)
}

// DeleteRelay - deletes a relay
func DeleteRelay(network, nodeid string) ([]models.Client, models.Node, error) {
	var returnClients []models.Client
	node, err := GetNodeByID(nodeid)
	if err != nil {
		return returnClients, models.Node{}, err
	}

	returnClients = SetRelayedNodes(false, nodeid, node.RelayedNodes)
	node.IsRelay = false
	node.RelayedNodes = []string{}
	node.SetLastModified()
	data, err := json.Marshal(&node)
	if err != nil {
		return returnClients, models.Node{}, err
	}
	if err = database.Insert(nodeid, string(data), database.NODES_TABLE_NAME); err != nil {
		return returnClients, models.Node{}, err
	}
	return returnClients, node, nil
}

func getRelayedAddresses(id string) []net.IPNet {
	addrs := []net.IPNet{}
	node, err := GetNodeByID(id)
	if err != nil {
		logger.Log(0, "getRelayedAddresses: "+err.Error())
		return addrs
	}
	if node.Address.IP != nil {
		addrs = append(addrs, net.IPNet{
			IP:   node.Address.IP,
			Mask: net.CIDRMask(32, 32),
		})
	}
	if node.Address6.IP != nil {
		addrs = append(addrs, net.IPNet{
			IP:   node.Address6.IP,
			Mask: net.CIDRMask(128, 128),
		})
	}
	return addrs
}
