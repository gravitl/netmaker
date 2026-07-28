package logic

import (
	"errors"
	"fmt"
	"net"
	"slices"
	"sort"
	"time"

	"context"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"golang.org/x/exp/slog"
	"gorm.io/gorm"
)

var (
	IPv4Network = "0.0.0.0/0"
	IPv6Network = "::/0"
)

var ErrIngressLimitExceeded = errors.New("gateway limit reached for this tenant, please upgrade your license")

var IngressLimitExceeded = func(ctx context.Context) bool {
	return false
}

// IsInternetGw - checks if node is acting as internet gw (legacy flag or internet egress router)
func IsInternetGw(node models.Node) bool {
	if node.IsInternetGateway {
		return true
	}
	return NodeIsInternetEgressRouter(node.ID.String(), node.Network)
}

// CreateEgressGateway - creates an egress gateway
func CreateEgressGateway(gateway models.EgressGatewayRequest) (models.Node, error) {
	node, err := GetNodeByID(gateway.NodeID)
	if err != nil {
		return models.Node{}, err
	}
	host := &schema.Host{
		ID: node.HostID,
	}
	err = host.Get(db.WithContext(context.TODO()))
	if err != nil {
		return models.Node{}, err
	}
	if host.OS != "linux" { // support for other OS to be added
		return models.Node{}, errors.New(host.OS + " is unsupported for egress gateways")
	}
	if host.FirewallInUse == schema.FIREWALL_NONE {
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

// GetIngressGwUsers - lists the users having to access to ingressGW
func GetIngressGwUsers(node models.Node) (models.IngressGwUsers, error) {
	gwUsers := models.IngressGwUsers{
		NodeID:  node.ID.String(),
		Network: node.Network,
	}

	ctx := scope.WithContext(db.WithContext(context.TODO()), scope.TenantScope, node.TenantID)
	_users, err := (&schema.User{}).ListAllWithMembership(ctx)
	if err != nil {
		return gwUsers, err
	}
	for _, _user := range _users {
		if _user.PlatformRoleID != schema.SuperAdminRole && _user.PlatformRoleID != schema.AdminRole {
			gwUsers.Users = append(gwUsers.Users, ToReturnUser(&_user))
		}
	}
	return gwUsers, nil
}

// DeleteIngressGateway - deletes an ingress gateway
func DeleteIngressGateway(ctx context.Context, nodeid string) (models.Node, []models.ExtClient, error) {
	removedClients := []models.ExtClient{}
	node, err := GetNodeByID(nodeid)
	if err != nil {
		return models.Node{}, removedClients, err
	}
	clients, err := GetExtClientsByID(ctx, nodeid, node.Network)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Node{}, removedClients, err
	}

	removedClients = clients

	// delete ext clients belonging to ingress gateway
	if err = DeleteGatewayExtClients(ctx, node.ID.String(), node.Network); err != nil {
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
	err = SetNetworkNodesLastModified(ctx, node.Network)
	return node, removedClients, err
}

// DeleteGatewayExtClients - deletes ext clients based on gateway (mac) of ingress node and network
func DeleteGatewayExtClients(ctx context.Context, gatewayID string, networkName string) error {
	currentExtClients, err := GetNetworkExtClients(ctx, networkName)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, extClient := range currentExtClients {
		if extClient.IngressGatewayID == gatewayID {
			if err = DeleteExtClient(ctx, networkName, extClient.ClientID, false); err != nil {
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
	user := &schema.User{Username: username}
	err := user.Get(db.WithContext(context.TODO()))
	if err != nil {
		return false
	}
	if user.Username != client.OwnerID {
		return false
	}
	return true
}

func ValidateInetGwReq(ctx context.Context, node *schema.Node, req models.InetNodeReq, update bool) error {
	_ = update // retained for callers; same-exit clients are allowed regardless of create vs update
	if node.Host.FirewallInUse == schema.FIREWALL_NONE {
		return errors.New("iptables or nftables needs to be installed")
	}
	if node.IsIGWClient || node.SelectedInternetEgressID != "" {
		return fmt.Errorf("node %s is using a internet gateway already", node.Host.Name)
	}
	if node.RelayedByNodeID != nil {
		return fmt.Errorf("node %s is being relayed", node.Host.Name)
	}

	for _, clientNodeID := range req.InetNodeClientIDs {
		clientNode, err := GetNodeByID(clientNodeID)
		if err != nil {
			return err
		}
		if clientNode.IsAutoRelay {
			return errors.New("failover node cannot be set to use internet gateway")
		}
		clientHost := &schema.Host{
			ID: clientNode.HostID,
		}
		err = clientHost.Get(ctx)
		if err != nil {
			return err
		}
		if clientHost.IsDefault {
			return errors.New("default host cannot be set to use internet gateway")
		}
		if clientNode.IsInternetGateway {
			return fmt.Errorf("node %s acting as internet gateway cannot use another internet gateway", clientHost.Name)
		}
		// Allow clients already using THIS node as their exit (SelectedInternetEgressID /
		// legacy InternetGwID). Only reject when they are assigned to a different exit.
		if exitID := InternetExitRoutingNodeID(&clientNode); exitID != "" && exitID != node.ID {
			return fmt.Errorf("node %s is already using a internet gateway", clientHost.Name)
		}
		if len(clientNode.AutoRelayedPeers) > 0 {
			ResetAutoRelayedPeer(ctx, &clientNode)
		}

		if clientNode.IsRelayed && clientNode.RelayedBy != node.ID {
			return fmt.Errorf("node %s is being relayed", clientHost.Name)
		}

		for _, clientID := range clientHost.Nodes {
			otherNode, err := GetNodeByID(clientID)
			if err != nil {
				continue
			}
			if otherNode.InternetGwID != "" && otherNode.InternetGwID != node.ID {
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
		if clientNode.AutoAssignGateway {
			clientNode.AutoAssignGateway = false
			if clientNode.RelayedBy != "" && clientNode.RelayedBy != node.ID.String() {
				currRelay, err := GetNodeByID(clientNode.RelayedBy)
				if err == nil {
					newRelayed := RemoveAllFromSlice(currRelay.RelayedNodes, clientNode.ID.String())
					UpdateRelayNodes(currRelay.ID.String(), currRelay.RelayedNodes, newRelayed)
				}
				clientNode.RelayedBy = ""
			}
		}
		clientNode.InternetGwID = node.ID.String()
		UpsertNode(&clientNode)
	}
}

func UnsetInternetGw(ctx context.Context, node *models.Node) {
	nodes, err := GetNetworkNodes(ctx, node.Network)
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

func SetDefaultGw(node models.Node, peerUpdate models.HostPeerUpdate) models.HostPeerUpdate {
	inetNodeID := InternetExitRoutingNodeID(&node)
	if inetNodeID == "" {
		return peerUpdate
	}

	inetNode, err := GetNodeByID(inetNodeID)
	if err != nil {
		return peerUpdate
	}
	// Fail open when the exit routing node is disconnected: do not keep pointing the
	// client's OS default route at a dead overlay next-hop.
	if !inetNode.Connected {
		return peerUpdate
	}

	gw4, gw6 := internetEgressGwIPs(&inetNode)
	if gw4 == nil && gw6 == nil {
		return peerUpdate
	}

	peerUpdate.ChangeDefaultGw = true
	peerUpdate.DefaultGwIp6 = gw6
	peerUpdate.DefaultGwIp = gw4
	// Legacy DefaultGwIp field: IPv4 preferred, else IPv6.
	if peerUpdate.DefaultGwIp == nil {
		peerUpdate.DefaultGwIp = peerUpdate.DefaultGwIp6
	}
	return peerUpdate
}

// internetEgressGwIPs returns overlay nexthops for a client using inetNode as exit,
// gated on the exit host's public endpoints (dual-stack = both EndpointIP and EndpointIPv6 present).
func internetEgressGwIPs(inetNode *models.Node) (gw4, gw6 net.IP) {
	if inetNode == nil {
		return nil, nil
	}
	host, ok := getExitHostSafe(inetNode.HostID)
	if !ok {
		// Safe fallback: use overlay addresses when host lookup fails.
		return inetNode.Address.IP, inetNode.Address6.IP
	}
	if len(host.EndpointIP) > 0 {
		gw4 = inetNode.Address.IP
	}
	if len(host.EndpointIPv6) > 0 {
		gw6 = inetNode.Address6.IP
	}
	return gw4, gw6
}

// exitHostHasEndpointIPv6 reports whether the exit node's host has a public IPv6 endpoint.
// Used to decide whether internet egress should expand ::/0.
func exitHostHasEndpointIPv6(node *models.Node) bool {
	if node == nil {
		return false
	}
	host, ok := getExitHostSafe(node.HostID)
	if !ok {
		return node.Address6.IP != nil
	}
	return len(host.EndpointIPv6) > 0
}

// getExitHostSafe loads a host without panicking when the DB is uninitialized (unit tests).
func getExitHostSafe(hostID uuid.UUID) (host *schema.Host, ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
			host = nil
		}
	}()
	h := &schema.Host{ID: hostID}
	if err := h.Get(db.WithContext(context.TODO())); err != nil {
		return nil, false
	}
	return h, true
}

// GetAllowedIpForInetNodeClient - get inet cidr for node using a inet gw.
// Dual-stack is decided from the exit peer host's public endpoints.
func GetAllowedIpForInetNodeClient(node, peer *models.Node) []net.IPNet {
	var allowedips = []net.IPNet{}
	gw4, gw6 := internetEgressGwIPs(peer)

	if len(gw4) > 0 {
		_, ipnet, _ := net.ParseCIDR(IPv4Network)
		allowedips = append(allowedips, *ipnet)
	}

	if len(gw6) > 0 {
		_, ipnet, _ := net.ParseCIDR(IPv6Network)
		allowedips = append(allowedips, *ipnet)
	}

	return allowedips
}
