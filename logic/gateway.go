package logic

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
)

// CreateEgressGateway - creates an egress gateway
func CreateEgressGateway(gateway models.EgressGatewayRequest) (models.Node, error) {
	node, err := GetNodeByID(gateway.NodeID)
	if node.OS == "windows" || node.OS == "macos" { // add in darwin later
		return models.Node{}, errors.New(node.OS + " is unsupported for egress gateways")
	}
	if err != nil {
		return models.Node{}, err
	}
	err = ValidateEgressGateway(gateway)
	if err != nil {
		return models.Node{}, err
	}
	node.IsEgressGateway = "yes"
	node.EgressGatewayRanges = gateway.Ranges
	postUpCmd := "iptables -A FORWARD -i " + node.Interface + " -j ACCEPT; iptables -t nat -A POSTROUTING -o " + gateway.Interface + " -j MASQUERADE"
	postDownCmd := "iptables -D FORWARD -i " + node.Interface + " -j ACCEPT; iptables -t nat -D POSTROUTING -o " + gateway.Interface + " -j MASQUERADE"
	if gateway.PostUp != "" {
		postUpCmd = gateway.PostUp
	}
	if gateway.PostDown != "" {
		postDownCmd = gateway.PostDown
	}
	if node.PostUp != "" {
		if !strings.Contains(node.PostUp, postUpCmd) {
			postUpCmd = node.PostUp + "; " + postUpCmd
		}
	}
	if node.PostDown != "" {
		if !strings.Contains(node.PostDown, postDownCmd) {
			postDownCmd = node.PostDown + "; " + postDownCmd
		}
	}
	key, err := GetRecordKey(gateway.NodeID, gateway.NetID)
	if err != nil {
		return node, err
	}
	node.PostUp = postUpCmd
	node.PostDown = postDownCmd
	node.SetLastModified()
	node.PullChanges = "yes"
	nodeData, err := json.Marshal(&node)
	if err != nil {
		return node, err
	}
	if err = database.Insert(key, string(nodeData), database.NODES_TABLE_NAME); err != nil {
		return models.Node{}, err
	}
	if err = NetworkNodesUpdatePullChanges(node.Network); err != nil {
		return models.Node{}, err
	}
	return node, nil
}

func ValidateEgressGateway(gateway models.EgressGatewayRequest) error {
	var err error

	empty := len(gateway.Ranges) == 0
	if empty {
		err = errors.New("IP Ranges Cannot Be Empty")
	}
	empty = gateway.Interface == ""
	if empty {
		err = errors.New("Interface cannot be empty")
	}
	return err
}

// DeleteEgressGateway - deletes egress from node
func DeleteEgressGateway(network, nodeid string) (models.Node, error) {

	node, err := GetNodeByID(nodeid)
	if err != nil {
		return models.Node{}, err
	}

	node.IsEgressGateway = "no"
	node.EgressGatewayRanges = []string{}
	node.PostUp = ""
	node.PostDown = ""
	if node.IsIngressGateway == "yes" { // check if node is still an ingress gateway before completely deleting postdown/up rules
		node.PostUp = "iptables -A FORWARD -i " + node.Interface + " -j ACCEPT; iptables -t nat -A POSTROUTING -o " + node.Interface + " -j MASQUERADE"
		node.PostDown = "iptables -D FORWARD -i " + node.Interface + " -j ACCEPT; iptables -t nat -D POSTROUTING -o " + node.Interface + " -j MASQUERADE"
	}
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

// CreateIngressGateway - creates an ingress gateway
func CreateIngressGateway(netid string, nodeid string) (models.Node, error) {

	node, err := GetNodeByID(nodeid)
	if node.OS == "windows" || node.OS == "darwin" { // add in darwin later
		return models.Node{}, errors.New(node.OS + " is unsupported for ingress gateways")
	}

	if err != nil {
		return models.Node{}, err
	}

	network, err := GetParentNetwork(netid)
	if err != nil {
		return models.Node{}, err
	}
	node.IsIngressGateway = "yes"
	node.IngressGatewayRange = network.AddressRange
	postUpCmd := "iptables -A FORWARD -i " + node.Interface + " -j ACCEPT; iptables -t nat -A POSTROUTING -o " + node.Interface + " -j MASQUERADE"
	postDownCmd := "iptables -D FORWARD -i " + node.Interface + " -j ACCEPT; iptables -t nat -D POSTROUTING -o " + node.Interface + " -j MASQUERADE"
	if node.PostUp != "" {
		if !strings.Contains(node.PostUp, postUpCmd) {
			postUpCmd = node.PostUp + "; " + postUpCmd
		}
	}
	if node.PostDown != "" {
		if !strings.Contains(node.PostDown, postDownCmd) {
			postDownCmd = node.PostDown + "; " + postDownCmd
		}
	}
	node.SetLastModified()
	node.PostUp = postUpCmd
	node.PostDown = postDownCmd
	node.PullChanges = "yes"
	node.UDPHolePunch = "no"
	key, err := GetRecordKey(node.MacAddress, node.Network)
	if err != nil {
		return models.Node{}, err
	}
	data, err := json.Marshal(&node)
	if err != nil {
		return models.Node{}, err
	}
	err = database.Insert(key, string(data), database.NODES_TABLE_NAME)
	if err != nil {
		return models.Node{}, err
	}
	err = SetNetworkNodesLastModified(netid)
	return node, err
}

// DeleteIngressGateway - deletes an ingress gateway
func DeleteIngressGateway(networkName string, macaddress string) (models.Node, error) {

	node, err := GetNodeByMacAddress(networkName, macaddress)
	if err != nil {
		return models.Node{}, err
	}
	network, err := GetParentNetwork(networkName)
	if err != nil {
		return models.Node{}, err
	}
	// delete ext clients belonging to ingress gateway
	if err = DeleteGatewayExtClients(macaddress, networkName); err != nil {
		return models.Node{}, err
	}

	node.UDPHolePunch = network.DefaultUDPHolePunch
	node.LastModified = time.Now().Unix()
	node.IsIngressGateway = "no"
	node.IngressGatewayRange = ""
	node.PullChanges = "yes"

	key, err := GetRecordKey(node.MacAddress, node.Network)
	if err != nil {
		return models.Node{}, err
	}
	data, err := json.Marshal(&node)
	if err != nil {
		return models.Node{}, err
	}
	err = database.Insert(key, string(data), database.NODES_TABLE_NAME)
	if err != nil {
		return models.Node{}, err
	}
	err = SetNetworkNodesLastModified(networkName)
	return node, err
}

// DeleteGatewayExtClients - deletes ext clients based on gateway (mac) of ingress node and network
func DeleteGatewayExtClients(gatewayID string, networkName string) error {
	currentExtClients, err := GetNetworkExtClients(networkName)
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}
	for _, extClient := range currentExtClients {
		if extClient.IngressGatewayID == gatewayID {
			if err = DeleteExtClient(networkName, extClient.ClientID); err != nil {
				logger.Log(1, "failed to remove ext client", extClient.ClientID)
				continue
			}
		}
	}
	return nil
}
