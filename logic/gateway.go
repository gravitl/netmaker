package logic

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

// IsInternetGw - checks if node is acting as internet gw
func IsInternetGw(node models.Node) bool {
	return node.IsInternetGateway
}

// GetInternetGateways - gets all the nodes that are internet gateways
func GetInternetGateways() ([]models.Node, error) {
	nodes, err := GetAllNodes()
	if err != nil {
		return nil, err
	}
	igs := make([]models.Node, 0)
	for _, node := range nodes {
		if node.IsInternetGateway {
			igs = append(igs, node)
		}
	}
	return igs, nil
}

// GetAllIngresses - gets all the nodes that are ingresses
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

// GetAllEgresses - gets all the nodes that are egresses
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
		return models.Node{}, errors.New("please install iptables or nftables on the device")
	}
	if len(gateway.RangesWithMetric) == 0 && len(gateway.Ranges) > 0 {
		for _, rangeI := range gateway.Ranges {
			gateway.RangesWithMetric = append(gateway.RangesWithMetric, models.EgressRangeMetric{
				Network:     rangeI,
				RouteMetric: 256,
			})
		}
	}
	for i := len(gateway.Ranges) - 1; i >= 0; i-- {
		// check if internet gateway IPv4
		if gateway.Ranges[i] == "0.0.0.0/0" || gateway.Ranges[i] == "::/0" {
			// remove inet range
			gateway.Ranges = append(gateway.Ranges[:i], gateway.Ranges[i+1:]...)
			continue
		}
		normalized, err := NormalizeCIDR(gateway.Ranges[i])
		if err != nil {
			return models.Node{}, err
		}
		gateway.Ranges[i] = normalized

	}
	rangesWithMetric := []string{}
	for i := len(gateway.RangesWithMetric) - 1; i >= 0; i-- {
		if gateway.RangesWithMetric[i].Network == "0.0.0.0/0" || gateway.RangesWithMetric[i].Network == "::/0" {
			// remove inet range
			gateway.RangesWithMetric = append(gateway.RangesWithMetric[:i], gateway.RangesWithMetric[i+1:]...)
			continue
		}
		normalized, err := NormalizeCIDR(gateway.RangesWithMetric[i].Network)
		if err != nil {
			return models.Node{}, err
		}
		gateway.RangesWithMetric[i].Network = normalized
		rangesWithMetric = append(rangesWithMetric, gateway.RangesWithMetric[i].Network)
		if gateway.RangesWithMetric[i].RouteMetric <= 0 || gateway.RangesWithMetric[i].RouteMetric > 999 {
			gateway.RangesWithMetric[i].RouteMetric = 256
		}
	}
	sort.Strings(gateway.Ranges)
	sort.Strings(rangesWithMetric)
	if !slices.Equal(gateway.Ranges, rangesWithMetric) {
		return models.Node{}, errors.New("invalid ranges")
	}
	if gateway.NatEnabled == "" {
		gateway.NatEnabled = "yes"
	}
	err = ValidateEgressGateway(gateway)
	if err != nil {
		return models.Node{}, err
	}
	if gateway.Ranges == nil {
		gateway.Ranges = make([]string, 0)
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
	return nil
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
		return models.Node{}, errors.New("gateway cannot be created on a relayed node")
	}
	host, err := GetHost(node.HostID.String())
	if err != nil {
		return models.Node{}, err
	}
	if host.OS != "linux" {
		return models.Node{}, errors.New("gateway can only be created on linux based node")
	}

	network, err := GetParentNetwork(netid)
	if err != nil {
		return models.Node{}, err
	}
	node.IsIngressGateway = true
	node.IsGw = true
	if !servercfg.IsPro {
		node.IsInternetGateway = ingress.IsInternetGateway
	}
	node.IngressGatewayRange = network.AddressRange
	node.IngressGatewayRange6 = network.AddressRange6
	node.IngressDNS = ingress.ExtclientDNS
	if node.IsInternetGateway && node.IngressDNS == "" {
		node.IngressDNS = "1.1.1.1"
	}
	node.IngressPersistentKeepalive = 20
	if ingress.PersistentKeepalive != 0 {
		node.IngressPersistentKeepalive = ingress.PersistentKeepalive
	}
	node.IngressMTU = 1420
	if ingress.MTU != 0 {
		node.IngressMTU = ingress.MTU
	}
	if servercfg.IsPro {
		if _, exists := FailOverExists(node.Network); exists {
			ResetFailedOverPeer(&node)
		}
	}
	node.SetLastModified()
	node.Metadata = ingress.Metadata
	if node.Metadata == "" {
		node.Metadata = "This host can be used for remote access"
	}
	if node.Tags == nil {
		node.Tags = make(map[models.TagID]struct{})
	}
	node.Tags[models.TagID(fmt.Sprintf("%s.%s", netid, models.GwTagName))] = struct{}{}
	err = UpsertNode(&node)
	if err != nil {
		return models.Node{}, err
	}
	err = SetNetworkNodesLastModified(netid)
	return node, err
}

// GetIngressGwUsers - lists the users having to access to ingressGW
func GetIngressGwUsers(node models.Node) (models.IngressGwUsers, error) {

	gwUsers := models.IngressGwUsers{
		NodeID:  node.ID.String(),
		Network: node.Network,
	}
	users, err := GetUsers()
	if err != nil {
		return gwUsers, err
	}
	for _, user := range users {
		if !user.IsAdmin && !user.IsSuperAdmin {
			gwUsers.Users = append(gwUsers.Users, user)
		}
	}
	return gwUsers, nil
}

// DeleteIngressGateway - deletes an ingress gateway
func DeleteIngressGateway(nodeid string) (models.Node, []models.ExtClient, error) {
	removedClients := []models.ExtClient{}
	node, err := GetNodeByID(nodeid)
	if err != nil {
		return models.Node{}, removedClients, err
	}
	clients, err := GetExtClientsByID(nodeid, node.Network)
	if err != nil && !database.IsEmptyRecord(err) {
		return models.Node{}, removedClients, err
	}

	removedClients = clients

	// delete ext clients belonging to ingress gateway
	if err = DeleteGatewayExtClients(node.ID.String(), node.Network); err != nil {
		return models.Node{}, removedClients, err
	}
	logger.Log(3, "deleting ingress gateway")
	node.LastModified = time.Now()
	node.IsIngressGateway = false
	if !servercfg.IsPro {
		node.IsInternetGateway = false
	}
	delete(node.Tags, models.TagID(fmt.Sprintf("%s.%s", node.Network, models.GwTagName)))
	node.IngressGatewayRange = ""
	node.Metadata = ""
	err = UpsertNode(&node)
	if err != nil {
		return models.Node{}, removedClients, err
	}

	err = SetNetworkNodesLastModified(node.Network)
	return node, removedClients, err
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

// IsUserAllowedAccessToExtClient - checks if user has permission to access extclient
func IsUserAllowedAccessToExtClient(username string, client models.ExtClient) bool {
	if username == MasterUser {
		return true
	}
	user, err := GetUser(username)
	if err != nil {
		return false
	}
	if user.UserName != client.OwnerID {
		return false
	}
	return true
}
