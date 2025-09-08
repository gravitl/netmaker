package logic

import (
	"errors"
	"fmt"
	"net"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
)

var (
	IPv4Network = "0.0.0.0/0"
	IPv6Network = "::/0"
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
		if node.EgressDetails.IsEgressGateway {
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
	node.EgressDetails.IsEgressGateway = true
	node.EgressDetails.EgressGatewayRanges = gateway.Ranges
	node.EgressDetails.EgressGatewayNatEnabled = models.ParseBool(gateway.NatEnabled)

	node.EgressDetails.EgressGatewayRequest = gateway // store entire request for use when preserving the egress gateway
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
	node.EgressDetails.IsEgressGateway = false
	node.EgressDetails.EgressGatewayRanges = []string{}
	node.EgressDetails.EgressGatewayRequest = models.EgressGatewayRequest{} // remove preserved request as the egress gateway is gone
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
	node.IsInternetGateway = ingress.IsInternetGateway
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
		if user.PlatformRoleID != models.SuperAdminRole && user.PlatformRoleID != models.AdminRole {
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
	node.LastModified = time.Now().UTC()
	node.IsIngressGateway = false
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
			if err = DeleteExtClient(networkName, extClient.ClientID, false); err != nil {
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

func ValidateInetGwReq(inetNode models.Node, req models.InetNodeReq, update bool) error {
	inetHost, err := GetHost(inetNode.HostID.String())
	if err != nil {
		return err
	}
	if inetHost.FirewallInUse == models.FIREWALL_NONE {
		return errors.New("iptables or nftables needs to be installed")
	}
	if inetNode.InternetGwID != "" {
		return fmt.Errorf("node %s is using a internet gateway already", inetHost.Name)
	}
	if inetNode.IsRelayed {
		return fmt.Errorf("node %s is being relayed", inetHost.Name)
	}

	for _, clientNodeID := range req.InetNodeClientIDs {
		clientNode, err := GetNodeByID(clientNodeID)
		if err != nil {
			return err
		}
		if clientNode.IsFailOver {
			return errors.New("failover node cannot be set to use internet gateway")
		}
		clientHost, err := GetHost(clientNode.HostID.String())
		if err != nil {
			return err
		}
		if clientHost.IsDefault {
			return errors.New("default host cannot be set to use internet gateway")
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
		if clientNode.FailedOverBy != uuid.Nil {
			ResetFailedOverPeer(&clientNode)
		}

		if clientNode.IsRelayed && clientNode.RelayedBy != inetNode.ID.String() {
			return fmt.Errorf("node %s is being relayed", clientHost.Name)
		}

		for _, nodeID := range clientHost.Nodes {
			node, err := GetNodeByID(nodeID)
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
		clientNode, err := GetNodeByID(clientNodeID)
		if err != nil {
			continue
		}
		clientNode.InternetGwID = node.ID.String()
		UpsertNode(&clientNode)
	}

}

func UnsetInternetGw(node *models.Node) {
	nodes, err := GetNetworkNodes(node.Network)
	if err != nil {
		slog.Error("failed to get network nodes", "network", node.Network, "error", err)
		return
	}
	for _, clientNode := range nodes {
		if node.ID.String() == clientNode.InternetGwID {
			clientNode.InternetGwID = ""
			UpsertNode(&clientNode)
		}

	}
	node.IsInternetGateway = false
	node.InetNodeReq = models.InetNodeReq{}

}

func SetDefaultGwForRelayedUpdate(relayed, relay models.Node, peerUpdate models.HostPeerUpdate) models.HostPeerUpdate {
	if relay.InternetGwID != "" {
		relayedHost, err := GetHost(relayed.HostID.String())
		if err != nil {
			return peerUpdate
		}
		peerUpdate.ChangeDefaultGw = true
		peerUpdate.DefaultGwIp = relay.Address.IP
		if peerUpdate.DefaultGwIp == nil || relayedHost.EndpointIP == nil {
			peerUpdate.DefaultGwIp = relay.Address6.IP
		}

	}
	return peerUpdate
}

func SetDefaultGw(node models.Node, peerUpdate models.HostPeerUpdate) models.HostPeerUpdate {
	if node.InternetGwID != "" {

		inetNode, err := GetNodeByID(node.InternetGwID)
		if err != nil {
			return peerUpdate
		}
		host, err := GetHost(node.HostID.String())
		if err != nil {
			return peerUpdate
		}

		peerUpdate.ChangeDefaultGw = true
		peerUpdate.DefaultGwIp = inetNode.Address.IP
		if peerUpdate.DefaultGwIp == nil || host.EndpointIP == nil {
			peerUpdate.DefaultGwIp = inetNode.Address6.IP
		}
	}
	return peerUpdate
}

// GetAllowedIpForInetNodeClient - get inet cidr for node using a inet gw
func GetAllowedIpForInetNodeClient(node, peer *models.Node) []net.IPNet {
	var allowedips = []net.IPNet{}

	if peer.Address.IP != nil {
		_, ipnet, _ := net.ParseCIDR(IPv4Network)
		allowedips = append(allowedips, *ipnet)
	}

	if peer.Address6.IP != nil {
		_, ipnet, _ := net.ParseCIDR(IPv6Network)
		allowedips = append(allowedips, *ipnet)
	}

	return allowedips
}
