package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic/acls/nodeacls"
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
		host, err := GetHost(node.HostID.String())
		if err == nil {
			returnnodes = append(returnnodes, models.Client{
				Host: *host,
				Node: node,
			})
		}
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

// GetPeerConfForRelayed - returns the peerConfig for a node relayed by relay
func GetPeerConfForRelayed(relayed, relay *models.Client) wgtypes.PeerConfig {
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
		if nodeacls.AreNodesAllowed(nodeacls.NetworkID(relayed.Node.Network), nodeacls.NodeID(relayed.Node.ID.String()), nodeacls.NodeID(peer.Node.ID.String())) {
			update.AllowedIPs = append(update.AllowedIPs, GetAllowedIPs(&peer)...)
		}
	}
	return update
}
