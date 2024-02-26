package logic

import (
	"errors"
	"fmt"
	"net"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"golang.org/x/exp/slog"
)

func ValidateInetGwReq(inetNode models.Node, req models.InetNodeReq, update bool) error {
	for _, clientNodeID := range req.InetNodeClientIDs {
		clientNode, err := logic.GetNodeByID(clientNodeID)
		if err != nil {
			return err
		}
		clientHost, err := logic.GetHost(clientNode.HostID.String())
		if err != nil {
			return err
		}
		if clientHost.OS != models.OS_Types.Linux && clientHost.OS != models.OS_Types.Windows {
			return errors.New("can only attach linux or windows machine to a internet gateway")
		}
		if clientNode.IsInternetGateway {
			return fmt.Errorf("node %s acting as internet gateway cannot use another internet gateway", clientHost.Name)
		}
		if update {
			if clientNode.InternetGwID != "" && clientNode.InternetGwID != inetNode.ID.String() {
				return fmt.Errorf("node %s is already using a internet gateway", clientHost.Name)
			}
		} else {
			if clientNode.InternetGwID != "" {
				return fmt.Errorf("node %s is already using a internet gateway", clientHost.Name)
			}
		}

		if clientNode.IsRelayed {
			return fmt.Errorf("node %s is being relayed", clientHost.Name)
		}

		for _, nodeID := range clientHost.Nodes {
			node, err := logic.GetNodeByID(nodeID)
			if err != nil {
				continue
			}
			if node.InternetGwID != "" && node.InternetGwID != inetNode.ID.String() {
				return errors.New("nodes on same host cannot use different internet gateway")
			}

		}
	}
	return nil
}

// SetInternetGw - sets the node as internet gw based on flag bool
func SetInternetGw(node *models.Node, req models.InetNodeReq) {
	node.IsInternetGateway = true
	node.InetNodeReq = req
	for _, clientNodeID := range req.InetNodeClientIDs {
		clientNode, err := logic.GetNodeByID(clientNodeID)
		if err != nil {
			continue
		}
		clientNode.InternetGwID = node.ID.String()
		logic.UpsertNode(&clientNode)
	}

}

func UnsetInternetGw(node *models.Node) {
	nodes, err := logic.GetNetworkNodes(node.Network)
	if err != nil {
		slog.Error("failed to get network nodes", "network", node.Network, "error", err)
		return
	}
	for _, clientNode := range nodes {
		if node.ID.String() == clientNode.InternetGwID {
			clientNode.InternetGwID = ""
			logic.UpsertNode(&clientNode)
		}

	}
	node.IsInternetGateway = false
	node.InetNodeReq = models.InetNodeReq{}

}

func SetDefaultGwForRelayedUpdate(relayed, relay models.Node, peerUpdate models.HostPeerUpdate) models.HostPeerUpdate {
	if relay.InternetGwID != "" {
		peerUpdate.ChangeDefaultGw = true
		peerUpdate.DefaultGwIp = relay.Address.IP

	}
	return peerUpdate
}

func SetDefaultGw(node models.Node, peerUpdate models.HostPeerUpdate) models.HostPeerUpdate {
	if node.InternetGwID != "" {

		inetNode, err := logic.GetNodeByID(node.InternetGwID)
		if err != nil {
			return peerUpdate
		}
		peerUpdate.ChangeDefaultGw = true
		peerUpdate.DefaultGwIp = inetNode.Address.IP

	}
	return peerUpdate
}

// GetNetworkIngresses - gets the gateways of a network
func GetNetworkIngresses(network string) ([]models.Node, error) {
	var ingresses []models.Node
	netNodes, err := logic.GetNetworkNodes(network)
	if err != nil {
		return []models.Node{}, err
	}
	for i := range netNodes {
		if netNodes[i].IsIngressGateway {
			ingresses = append(ingresses, netNodes[i])
		}
	}
	return ingresses, nil
}

// GetAllowedIpsForInet - get inet cidr for node using a inet gw
func GetAllowedIpForInetNodeClient(node, peer *models.Node) []net.IPNet {
	_, ipnet, _ := net.ParseCIDR("0.0.0.0/0")
	return []net.IPNet{*ipnet}
}
