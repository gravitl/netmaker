package logic

import (
	"encoding/json"
	"errors"
	"fmt"
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
	node.RelayAddrs = relay.RelayAddrs

	node.SetLastModified()
	nodeData, err := json.Marshal(&node)
	if err != nil {
		return returnnodes, node, err
	}
	// invalidate cache
	CacheNodesMutex.Lock()
	CacheNodes = nil
	CacheNodesMutex.Unlock()
	if err = database.Insert(node.ID.String(), string(nodeData), database.NODES_TABLE_NAME); err != nil {
		return returnnodes, models.Node{}, err
	}
	returnnodes, err = SetRelayedNodes(true, node.Network, node.RelayAddrs)
	if err != nil {
		return returnnodes, node, err
	}
	return returnnodes, node, nil
}

// CreateHostRelay - creates a host relay
func CreateHostRelay(relay models.HostRelayRequest) (relayHost *models.Host, relayedHosts []models.Host, err error) {

	relayHost, err = GetHost(relay.HostID)
	if err != nil {
		return
	}
	err = validateHostRelay(relay)
	if err != nil {
		return
	}
	relayHost.IsRelay = true
	relayHost.ProxyEnabled = true
	relayHost.RelayedHosts = relay.RelayedHosts
	err = UpsertHost(relayHost)
	if err != nil {
		return
	}
	relayedHosts = SetRelayedHosts(true, relay.HostID, relay.RelayedHosts)
	return
}

// SetRelayedHosts - updates the relayed hosts status
func SetRelayedHosts(setRelayed bool, relayHostID string, relayedHostIDs []string) []models.Host {
	var relayedHosts []models.Host
	for _, relayedHostID := range relayedHostIDs {
		host, err := GetHost(relayedHostID)
		if err == nil {
			if setRelayed {
				host.IsRelayed = true
				host.RelayedBy = relayHostID
				host.ProxyEnabled = true
			} else {
				host.IsRelayed = false
				host.RelayedBy = ""
			}
			err = UpsertHost(host)
			if err == nil {
				relayedHosts = append(relayedHosts, *host)
			}
		}
	}
	return relayedHosts
}

// SetRelayedNodes- set relayed nodes
func SetRelayedNodes(setRelayed bool, networkName string, addrs []string) ([]models.Node, error) {
	var returnnodes []models.Node
	networkNodes, err := GetNetworkNodes(networkName)
	if err != nil {
		return returnnodes, err
	}
	for _, node := range networkNodes {
		for _, addr := range addrs {
			if addr == node.Address.IP.String() || addr == node.Address6.IP.String() {
				if setRelayed {
					node.IsRelayed = true
				} else {
					node.IsRelayed = false
				}
				data, err := json.Marshal(&node)
				if err != nil {
					return returnnodes, err
				}
				// invalidate cache
				CacheNodesMutex.Lock()
				CacheNodes = nil
				CacheNodesMutex.Unlock()
				database.Insert(node.ID.String(), string(data), database.NODES_TABLE_NAME)
				returnnodes = append(returnnodes, node)
			}
		}
	}
	return returnnodes, nil
}
func GetRelayedNodes(relayNode *models.Node) ([]models.Node, error) {
	var returnnodes []models.Node
	networkNodes, err := GetNetworkNodes(relayNode.Network)
	if err != nil {
		return returnnodes, err
	}
	for _, node := range networkNodes {
		for _, addr := range relayNode.RelayAddrs {
			if addr == node.Address.IP.String() || addr == node.Address6.IP.String() {
				returnnodes = append(returnnodes, node)
			}
		}
	}
	return returnnodes, nil
}

// GetRelayedHosts - gets the relayed hosts of a relay host
func GetRelayedHosts(relayHost *models.Host) []models.Host {
	relayedHosts := []models.Host{}

	for _, hostID := range relayHost.RelayedHosts {
		relayedHost, err := GetHost(hostID)
		if err == nil {
			relayedHosts = append(relayedHosts, *relayedHost)
		}
	}
	return relayedHosts
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

func validateHostRelay(relay models.HostRelayRequest) error {
	if len(relay.RelayedHosts) == 0 {
		return errors.New("relayed hosts are empty")
	}
	return nil
}

// UpdateRelay - updates a relay
func UpdateRelay(network string, oldAddrs []string, newAddrs []string) []models.Node {
	var returnnodes []models.Node
	time.Sleep(time.Second / 4)
	_, err := SetRelayedNodes(false, network, oldAddrs)
	if err != nil {
		logger.Log(1, err.Error())
	}
	returnnodes, err = SetRelayedNodes(true, network, newAddrs)
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
	returnnodes, err = SetRelayedNodes(false, node.Network, node.RelayAddrs)
	if err != nil {
		return returnnodes, node, err
	}

	node.IsRelay = false
	node.RelayAddrs = []string{}
	node.SetLastModified()

	data, err := json.Marshal(&node)
	if err != nil {
		return returnnodes, models.Node{}, err
	}
	// invalidate cache
	CacheNodesMutex.Lock()
	CacheNodes = nil
	CacheNodesMutex.Unlock()
	if err = database.Insert(nodeid, string(data), database.NODES_TABLE_NAME); err != nil {
		return returnnodes, models.Node{}, err
	}
	return returnnodes, node, nil
}

// DeleteHostRelay - removes host as relay
func DeleteHostRelay(relayHostID string) (relayHost *models.Host, relayedHosts []models.Host, err error) {
	relayHost, err = GetHost(relayHostID)
	if err != nil {
		return
	}
	relayedHosts = SetRelayedHosts(false, relayHostID, relayHost.RelayedHosts)
	relayHost.IsRelay = false
	relayHost.RelayedHosts = []string{}
	err = UpsertHost(relayHost)
	if err != nil {
		return
	}
	return
}

// UpdateHostRelay - updates the relay host with new relayed hosts
func UpdateHostRelay(relayHostID string, oldRelayedHosts, newRelayedHosts []string) {
	_ = SetRelayedHosts(false, relayHostID, oldRelayedHosts)
	_ = SetRelayedHosts(true, relayHostID, newRelayedHosts)
}
