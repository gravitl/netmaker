package logic

import (
	"context"
	"net"
	"time"

	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"golang.org/x/exp/slog"
)

// GetNetworkStatus returns the status of every node and extclient in the network.
// When includePeers is false, per-peer details are omitted but peer counts are kept.
func GetNetworkStatus(ctx context.Context, network string, includePeers bool) (*models.NetworkStatus, error) {
	nodes, err := GetNetworkNodes(ctx, network)
	if err != nil {
		return nil, err
	}
	extClients, err := GetNetworkExtClients(ctx, network)
	if err != nil {
		return nil, err
	}
	for i := range extClients {
		staticNode := models.ConvertToStaticNode(extClients[i])
		staticNode.SelectedInternetEgressID = extClients[i].SelectedInternetEgressID
		nodes = append(nodes, staticNode)
	}
	nodes = AddStatusToNodes(ctx, nodes, true)

	eli, err := (&schema.Egress{Network: network}).ListByNetwork(db.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	hosts := networkStatusHosts(ctx, nodes)

	names := make(map[string]string, len(nodes))
	nodesByID := make(map[string]*models.Node, len(nodes))
	for i := range nodes {
		node := &nodes[i]
		if node.IsStatic {
			names[node.StaticNode.ClientID] = node.StaticNode.ClientID
			continue
		}
		nodesByID[node.ID.String()] = node
		if host, ok := hosts[node.HostID.String()]; ok {
			names[node.ID.String()] = host.Name
		}
	}

	status := &models.NetworkStatus{
		Network:     network,
		GeneratedAt: time.Now().UTC().Unix(),
		Nodes:       make([]models.NetworkNodeStatus, 0, len(nodes)),
	}
	inetRouters := InternetEgressRoutingNodeIDsFromList(eli)
	for i := range nodes {
		node := &nodes[i]
		var nodeStatus models.NetworkNodeStatus
		if node.IsStatic {
			nodeStatus = extClientNetworkStatus(ctx, node, eli, names, includePeers)
		} else {
			nodeStatus = nodeNetworkStatus(ctx, node, hosts[node.HostID.String()], eli, inetRouters, names, nodesByID, includePeers)
		}
		countNetworkNodeStatus(&status.Summary, nodeStatus.Status)
		status.Nodes = append(status.Nodes, nodeStatus)
	}
	return status, nil
}

// networkStatusHosts fetches the hosts of the given nodes in one query, keyed by host ID.
func networkStatusHosts(ctx context.Context, nodes []models.Node) map[string]schema.Host {
	hostIDs := make([]interface{}, 0, len(nodes))
	for i := range nodes {
		if !nodes[i].IsStatic {
			hostIDs = append(hostIDs, nodes[i].HostID.String())
		}
	}
	hosts := make(map[string]schema.Host, len(hostIDs))
	if len(hostIDs) == 0 {
		return hosts
	}
	_hosts, err := (&schema.Host{}).ListAll(ctx, dbtypes.WithFilter("id", hostIDs...))
	if err != nil {
		slog.Error("failed to list hosts for network status", "error", err)
		return hosts
	}
	for i := range _hosts {
		hosts[_hosts[i].ID.String()] = _hosts[i]
	}
	return hosts
}

func nodeNetworkStatus(
	ctx context.Context,
	node *models.Node,
	host schema.Host,
	eli []schema.Egress,
	inetRouters map[string]struct{},
	names map[string]string,
	nodesByID map[string]*models.Node,
	includePeers bool,
) models.NetworkNodeStatus {
	nodeID := node.ID.String()
	_, isInetRouter := inetRouters[nodeID]
	nodeStatus := models.NetworkNodeStatus{
		ID:                nodeID,
		Kind:              models.NetworkNodeKindNode,
		Name:              host.Name,
		HostID:            node.HostID.String(),
		MacAddress:        host.MacAddress.String(),
		Owner:             NodeOwnerUsername(node),
		Address:           ipNetAddr(node.Address),
		Address6:          ipNetAddr(node.Address6),
		OS:                host.OS,
		Version:           host.Version,
		IsGateway:         node.IsGw || node.IsIngressGateway || node.IsRelay,
		IsInternetGateway: isInetRouter || node.IsInternetGateway,
		IsEgress:          routesEgress(node, eli),
		Status:            node.Status,
		Connected:         node.Connected,
		LastCheckIn:       unixOrZero(node.LastCheckIn),
	}
	if IsUserOwnedDevice(node) {
		nodeStatus.Kind = models.NetworkNodeKindUser
	}
	if host.EndpointIP != nil {
		nodeStatus.EndpointIP = host.EndpointIP.String()
	} else if host.EndpointIPv6 != nil {
		nodeStatus.EndpointIP = host.EndpointIPv6.String()
	}
	if node.IsRelayed {
		nodeStatus.GatewayNodeID = node.RelayedBy
	}
	if e := assignedInternetEgress(node, eli); e != nil {
		nodeStatus.InternetGatewayNodeID = FirstInternetEgressRoutingNodeID(*e)
	}

	metrics, err := GetMetrics(ctx, nodeID)
	if err != nil || metrics == nil {
		return nodeStatus
	}
	nodeStatus.MetricsUpdatedAt = unixOrZero(metrics.UpdatedAt)
	for peerID, metric := range metrics.Connectivity {
		nodeStatus.TotalPeers++
		if metric.Connected {
			nodeStatus.ConnectedPeers++
		}
		if !includePeers {
			continue
		}
		peerStatus := peerNetworkStatus(peerID, metric, names)
		if relayID := peerRelayNodeID(node, nodesByID[peerID]); relayID != "" {
			peerStatus.ConnectionType = models.PeerConnectionRelayed
			peerStatus.RelayNodeID = relayID
		}
		nodeStatus.Peers = append(nodeStatus.Peers, peerStatus)
	}
	return nodeStatus
}

// extClientNetworkStatus reports an extclient's link to its ingress gateway,
// which is its only peer. The link metrics come from the gateway.
func extClientNetworkStatus(
	ctx context.Context,
	node *models.Node,
	eli []schema.Egress,
	names map[string]string,
	includePeers bool,
) models.NetworkNodeStatus {
	ext := node.StaticNode
	nodeStatus := models.NetworkNodeStatus{
		ID:            ext.ClientID,
		Kind:          models.NetworkNodeKindExtClient,
		Name:          ext.ClientID,
		MacAddress:    ext.RemoteAccessClientID,
		Owner:         ext.OwnerID,
		Address:       ext.Address,
		Address6:      ext.Address6,
		EndpointIP:    ext.PublicEndpoint,
		OS:            ext.OS,
		Version:       ext.ClientVersion,
		Status:        node.Status,
		Connected:     ext.Enabled,
		GatewayNodeID: ext.IngressGatewayID,
	}
	if node.IsUserNode {
		nodeStatus.Kind = models.NetworkNodeKindUser
	}
	if e := assignedInternetEgress(node, eli); e != nil {
		nodeStatus.InternetGatewayNodeID = FirstInternetEgressRoutingNodeID(*e)
	}
	if ext.IngressGatewayID == "" {
		return nodeStatus
	}
	metrics, err := GetMetrics(ctx, ext.IngressGatewayID)
	if err != nil || metrics == nil {
		return nodeStatus
	}
	metric, ok := metrics.Connectivity[ext.ClientID]
	if !ok {
		return nodeStatus
	}
	nodeStatus.MetricsUpdatedAt = unixOrZero(metrics.UpdatedAt)
	nodeStatus.TotalPeers = 1
	if metric.Connected {
		nodeStatus.ConnectedPeers = 1
	}
	if includePeers {
		peerStatus := peerNetworkStatus(ext.IngressGatewayID, metric, names)
		// The gateway records bytes from its own side; flip them to the extclient's.
		peerStatus.BytesSent, peerStatus.BytesReceived = metric.TotalReceived, metric.TotalSent
		nodeStatus.Peers = []models.NetworkPeerStatus{peerStatus}
	}
	return nodeStatus
}

func peerNetworkStatus(peerID string, metric models.Metric, names map[string]string) models.NetworkPeerStatus {
	name := names[peerID]
	if name == "" {
		name = metric.NodeName
	}
	return models.NetworkPeerStatus{
		PeerID:         peerID,
		Name:           name,
		Connected:      metric.Connected,
		LatencyMs:      metric.Latency,
		ConnectionType: models.PeerConnectionDirect,
		PercentUp:      metric.PercentUp,
		BytesSent:      metric.TotalSent,
		BytesReceived:  metric.TotalReceived,
	}
}

// peerRelayNodeID returns the node relaying traffic between node and peer,
// or "" when they connect directly. peer is nil for extclients and unknown peers.
func peerRelayNodeID(node, peer *models.Node) string {
	if peer == nil {
		return ""
	}
	peerID := peer.ID.String()
	if relayID, ok := node.AutoRelayedPeers[peerID]; ok && relayID != "" {
		return relayID
	}
	if node.IsRelayed && node.RelayedBy != "" && node.RelayedBy != peerID {
		return node.RelayedBy
	}
	if peer.IsRelayed && peer.RelayedBy != "" && peer.RelayedBy != node.ID.String() {
		return peer.RelayedBy
	}
	return ""
}

// routesEgress reports whether the node routes any active non-internet egress,
// either directly or through one of its tags.
func routesEgress(node *models.Node, eli []schema.Egress) bool {
	nodeID := node.ID.String()
	for _, e := range eli {
		if !e.Status || IsEgressInternetGateway(e) {
			continue
		}
		if _, ok := e.Nodes[nodeID]; ok {
			return true
		}
		for tagID := range node.Tags {
			if _, ok := e.Tags[tagID.String()]; ok {
				return true
			}
		}
	}
	return false
}

func countNetworkNodeStatus(summary *models.NetworkStatusSummary, status schema.NodeStatus) {
	summary.Total++
	switch status {
	case schema.OnlineSt:
		summary.Online++
	case schema.OfflineSt:
		summary.Offline++
	case schema.WarningSt:
		summary.Warning++
	case schema.ErrorSt:
		summary.Error++
	case schema.Disconnected:
		summary.Disconnected++
	default:
		summary.Unknown++
	}
}

func ipNetAddr(n net.IPNet) string {
	if n.IP == nil || n.IP.IsUnspecified() {
		return ""
	}
	return n.IP.String()
}

func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}
