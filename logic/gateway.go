package logic

import (
	"errors"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

// GetAllIngresses - gets all the hosts that are ingresses
func GetAllIngresses() ([]models.Node, error) {
	nodes, err := GetAllNodes()
	if err != nil {
		return nil, err
	}
	ingresses := make([]models.Node, 0)
	for _, node := range nodes {
		if node.IsIngressGateway {
			ingresses = append(ingresses, node)
		}
	}
	return ingresses, nil
}

// GetAllEgresses - gets all the hosts that are egresses
func GetAllEgresses() ([]models.Node, error) {
	nodes, err := GetAllNodes()
	if err != nil {
		return nil, err
	}
	egresses := make([]models.Node, 0)
	for _, node := range nodes {
		if node.IsEgressGateway {
			egresses = append(egresses, node)
		}
	}
	return egresses, nil
}

// CreateEgressGateway - creates an egress gateway
func CreateEgressGateway(gateway models.EgressGatewayRequest) (models.Node, error) {
	node, err := GetNodeByID(gateway.NodeID)
	if err != nil {
		return models.Node{}, err
	}
	host, err := GetHost(node.HostID.String())
	if err != nil {
		return models.Node{}, err
	}
	if host.OS != "linux" { // support for other OS to be added
		return models.Node{}, errors.New(host.OS + " is unsupported for egress gateways")
	}
	if host.FirewallInUse == models.FIREWALL_NONE {
		return models.Node{}, errors.New("firewall is not supported for egress gateways")
	}
	for i := len(gateway.Ranges) - 1; i >= 0; i-- {
		// check if internet gateway IPv4
		if gateway.Ranges[i] == "0.0.0.0/0" && FreeTier {
			logger.Log(0, "currently IPv4 internet gateways are not supported on the free tier", gateway.Ranges[i])
			gateway.Ranges = append(gateway.Ranges[:i], gateway.Ranges[i+1:]...)
			continue
		}
		// check if internet gateway IPv6
		if gateway.Ranges[i] == "::/0" {
			logger.Log(0, "currently IPv6 internet gateways are not supported", gateway.Ranges[i])
			gateway.Ranges = append(gateway.Ranges[:i], gateway.Ranges[i+1:]...)
			continue
		}
		normalized, err := NormalizeCIDR(gateway.Ranges[i])
		if err != nil {
			return models.Node{}, err
		}
		gateway.Ranges[i] = normalized

	}
	if gateway.NatEnabled == "" {
		gateway.NatEnabled = "yes"
	}
	err = ValidateEgressGateway(gateway)
	if err != nil {
		return models.Node{}, err
	}
	node.IsEgressGateway = true
	node.EgressGatewayRanges = gateway.Ranges
	node.EgressGatewayNatEnabled = models.ParseBool(gateway.NatEnabled)
	node.EgressGatewayRequest = gateway // store entire request for use when preserving the egress gateway
	node.SetLastModified()
	if err = UpsertNode(&node); err != nil {
		return models.Node{}, err
	}
	return node, nil
}

// ValidateEgressGateway - validates the egress gateway model
func ValidateEgressGateway(gateway models.EgressGatewayRequest) error {
	var err error

	empty := len(gateway.Ranges) == 0
	if empty {
		err = errors.New("IP Ranges Cannot Be Empty")
	}
	return err
}

// DeleteEgressGateway - deletes egress from node
func DeleteEgressGateway(network, nodeid string) (models.Node, error) {
	node, err := GetNodeByID(nodeid)
	if err != nil {
		return models.Node{}, err
	}
	node.IsEgressGateway = false
	node.EgressGatewayRanges = []string{}
	node.EgressGatewayRequest = models.EgressGatewayRequest{} // remove preserved request as the egress gateway is gone
	node.SetLastModified()
	if err = UpsertNode(&node); err != nil {
		return models.Node{}, err
	}
	return node, nil
}

// CreateIngressGateway - creates an ingress gateway
func CreateIngressGateway(netid string, nodeid string, ingress models.IngressRequest) (models.Node, error) {

	node, err := GetNodeByID(nodeid)
	if err != nil {
		return models.Node{}, err
	}
	if node.IsRelayed {
		return models.Node{}, errors.New("ingress cannot be created on a relayed node")
	}
	host, err := GetHost(node.HostID.String())
	if err != nil {
		return models.Node{}, err
	}
	if host.OS != "linux" {
		return models.Node{}, errors.New("ingress can only be created on linux based node")
	}
	if host.FirewallInUse == models.FIREWALL_NONE {
		return models.Node{}, errors.New("firewall is not supported for ingress gateways")
	}

	network, err := GetParentNetwork(netid)
	if err != nil {
		return models.Node{}, err
	}
	node.IsIngressGateway = true
	node.IngressGatewayRange = network.AddressRange
	node.IngressGatewayRange6 = network.AddressRange6
	node.IngressDNS = ingress.ExtclientDNS
	node.SetLastModified()
	if ingress.Failover && servercfg.Is_EE {
		node.Failover = true
	}
	err = UpsertNode(&node)
	if err != nil {
		return models.Node{}, err
	}
	err = SetNetworkNodesLastModified(netid)
	return node, err
}

// DeleteIngressGateway - deletes an ingress gateway
func DeleteIngressGateway(nodeid string) (models.Node, bool, []models.ExtClient, error) {
	removedClients := []models.ExtClient{}
	node, err := GetNodeByID(nodeid)
	if err != nil {
		return models.Node{}, false, removedClients, err
	}
	clients, err := GetExtClientsByID(nodeid, node.Network)
	if err != nil && !database.IsEmptyRecord(err) {
		return models.Node{}, false, removedClients, err
	}

	removedClients = clients

	// delete ext clients belonging to ingress gateway
	if err = DeleteGatewayExtClients(node.ID.String(), node.Network); err != nil {
		return models.Node{}, false, removedClients, err
	}
	logger.Log(3, "deleting ingress gateway")
	wasFailover := node.Failover
	node.LastModified = time.Now()
	node.IsIngressGateway = false
	node.IngressGatewayRange = ""
	node.Failover = false
	err = UpsertNode(&node)
	if err != nil {
		return models.Node{}, wasFailover, removedClients, err
	}
	err = SetNetworkNodesLastModified(node.Network)
	return node, wasFailover, removedClients, err
}

// DeleteGatewayExtClients - deletes ext clients based on gateway (mac) of ingress node and network
func DeleteGatewayExtClients(gatewayID string, networkName string) error {
	currentExtClients, err := GetNetworkExtClients(networkName)
	if database.IsEmptyRecord(err) {
		return nil
	}
	if err != nil {
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
