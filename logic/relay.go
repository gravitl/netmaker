package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
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
		err = errors.New("relayed nodes cannot be empty")
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
		node.Address.Mask = net.CIDRMask(32, 32)
		addrs = append(addrs, node.Address)
	}
	if node.Address6.IP != nil {
		node.Address6.Mask = net.CIDRMask(128, 128)
		addrs = append(addrs, node.Address6)
	}
	return addrs
}

// peerUpdateForRelayed - returns the peerConfig for a relayed node
func peerUpdateForRelayed(client *models.Client, peers *[]models.Client) []wgtypes.PeerConfig {
	peerConfig := []wgtypes.PeerConfig{}
	if !client.Node.IsRelayed {
		logger.Log(0, "GetPeerUpdateForRelayed called for non-relayed node ", client.Host.Name)
		return []wgtypes.PeerConfig{}
	}
	relayNode, err := GetNodeByID(client.Node.RelayedBy)
	if err != nil {
		logger.Log(0, "error retrieving relay node", err.Error())
		return []wgtypes.PeerConfig{}
	}
	relay := models.Client{
		Host: *GetHostByNodeID(relayNode.ID.String()),
		Node: relayNode,
	}
	for _, peer := range *peers {
		if peer.Host.ID == client.Host.ID {
			continue
		}
		if peer.Host.ID == relay.Host.ID { // add relay as a peer
			update := peerUpdateForRelayedByRelay(client, &relay)
			peerConfig = append(peerConfig, update)
			continue
		}
		update := wgtypes.PeerConfig{
			PublicKey: peer.Host.PublicKey,
			Remove:    true,
		}
		peerConfig = append(peerConfig, update)
	}
	return peerConfig
}

// peerUpdateForRelayedByRelay - returns the peerConfig for a node relayed by relay
func peerUpdateForRelayedByRelay(relayed, relay *models.Client) wgtypes.PeerConfig {
	if relayed.Node.RelayedBy != relay.Node.ID.String() {
		logger.Log(0, "peerUpdateForRelayedByRelay called with invalid parameters")
		return wgtypes.PeerConfig{}
	}
	update := wgtypes.PeerConfig{
		PublicKey:         relay.Host.PublicKey,
		ReplaceAllowedIPs: true,
		Endpoint: &net.UDPAddr{
			IP:   relay.Host.EndpointIP,
			Port: relay.Host.ListenPort,
		},
		PersistentKeepaliveInterval: &relay.Node.PersistentKeepalive,
	}
	if relay.Node.Address.IP != nil {
		relay.Node.Address.Mask = net.CIDRMask(32, 32)
		update.AllowedIPs = append(update.AllowedIPs, relay.Node.Address)
	}
	if relay.Node.Address6.IP != nil {
		relay.Node.Address6.Mask = net.CIDRMask(128, 128)
		update.AllowedIPs = append(update.AllowedIPs, relay.Node.Address6)
	}
	if relay.Node.IsEgressGateway {
		update.AllowedIPs = append(update.AllowedIPs, getEgressIPs(relay)...)
	}
	if relay.Node.IsIngressGateway {
		update.AllowedIPs = append(update.AllowedIPs, getIngressIPs(relay)...)
	}
	peers, err := GetNetworkClients(relay.Node.Network)
	if err != nil {
		logger.Log(0, "error getting network clients", err.Error())
		return update
	}
	for _, peer := range peers {
		if peer.Host.ID == relayed.Host.ID || peer.Host.ID == relay.Host.ID {
			continue
		}
		update.AllowedIPs = append(update.AllowedIPs, AddAllowedIPs(&peer)...)
	}
	return update
}

// peerUpdateForRelay - returns the peerConfig for a relay
func peerUpdateForRelay(relay *models.Client, peers *[]models.Client) []wgtypes.PeerConfig {
	peerConfig := []wgtypes.PeerConfig{}
	if !relay.Node.IsRelay {
		logger.Log(0, "GetPeerUpdateForRelay called for non-relay node ", relay.Host.Name)
		return []wgtypes.PeerConfig{}
	}
	for _, peer := range *peers {
		if peer.Host.ID == relay.Host.ID {
			continue
		}
		update := wgtypes.PeerConfig{
			PublicKey:         peer.Host.PublicKey,
			ReplaceAllowedIPs: true,
			Remove:            false,
			Endpoint: &net.UDPAddr{
				IP:   peer.Host.EndpointIP,
				Port: peer.Host.ListenPort,
			},
			PersistentKeepaliveInterval: &peer.Node.PersistentKeepalive,
		}
		update.AllowedIPs = append(update.AllowedIPs, AddAllowedIPs(&peer)...)
		peerConfig = append(peerConfig, update)
	}
	return peerConfig
}
