package logic

import (
	"errors"
	"fmt"
	"net"

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
	err = UpsertNode(&node)
	if err != nil {
		return relayedClients, node, err
	}
	returnnodes := SetRelayedNodes(true, relay.NodeID, relay.RelayedNodes)
	return returnnodes, node, nil
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
		if err := UpsertNode(&node); err != nil {
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
	if err = UpsertNode(&node); err != nil {
		return returnClients, models.Node{}, err
	}
	return returnClients, node, nil
}

// GetPeerConfForRelayed - returns the peerConfig for a node relayed by relay
func GetPeerConfForRelayed(relayed, relay models.Client) wgtypes.PeerConfig {
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
		AllowedIPs:                  getAllowedIpsForRelayed(relayed, relay),
		PersistentKeepaliveInterval: &relay.Node.PersistentKeepalive,
	}
	if relay.Node.Address.IP != nil {
		allowed := net.IPNet{
			IP:   relay.Node.Address.IP,
			Mask: net.CIDRMask(32, 32),
		}
		update.AllowedIPs = append(update.AllowedIPs, allowed)
	}
	if relay.Node.Address6.IP != nil {
		allowed := net.IPNet{
			IP:   relay.Node.Address6.IP,
			Mask: net.CIDRMask(128, 128),
		}
		update.AllowedIPs = append(update.AllowedIPs, allowed)
	}
	if relay.Node.IsIngressGateway {
		update.AllowedIPs = append(update.AllowedIPs, getIngressIPs(relay)...)
	}
	if relay.Node.IsEgressGateway {
		update.AllowedIPs = append(update.AllowedIPs, getEgressIPs(relay)...)
	}
	return update
}
