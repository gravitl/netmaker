package logic

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// ErrTopologyNodeNotFound is returned when the requested node is not in the network.
var ErrTopologyNodeNotFound = errors.New("node not found")

// GetNetworkTopology returns users, their devices, servers, and extclients.
// Peer lines are served by GetNodeConnections.
func GetNetworkTopology(ctx context.Context, network string) (*models.NetworkTopology, error) {
	b, nodes, err := loadNetworkStatusBuilder(ctx, network, NetworkStatusOptions{})
	if err != nil {
		return nil, err
	}
	b.indexStaticClients(nodes)
	nodes = AddStatusToNodes(ctx, nodes, true)
	return b.networkTopology(network, nodes), nil
}

// GetNodeConnections returns the ACL-allowed peer lines for one user device,
// server, or extclient. nodeID is a node ID or an extclient client ID.
func GetNodeConnections(ctx context.Context, network, nodeID string) (*models.NodeConnections, error) {
	b, nodes, err := loadNetworkStatusBuilder(ctx, network, NetworkStatusOptions{})
	if err != nil {
		return nil, err
	}
	b.indexStaticClients(nodes)
	return b.nodeConnections(nodes, nodeID)
}

func graphNodeID(node *models.Node) string {
	if node == nil {
		return ""
	}
	if node.IsStatic {
		return node.StaticNode.ClientID
	}
	return node.ID.String()
}

func (b *networkStatusBuilder) networkTopology(network string, nodes []models.Node) *models.NetworkTopology {
	topo := &models.NetworkTopology{
		Network:     network,
		GeneratedAt: time.Now().UTC().Unix(),
		Users:       []models.NetworkTopologyUser{},
		Servers:     []models.NetworkTopologyNode{},
		ExtClients:  []models.NetworkTopologyNode{},
	}
	byUser := map[string][]models.NetworkTopologyNode{}
	for i := range nodes {
		node := &nodes[i]
		vert := b.topologyNode(node)
		switch vert.Kind {
		case models.NetworkNodeKindUser:
			owner := NodeOwnerUsername(node)
			byUser[owner] = append(byUser[owner], vert)
		case models.NetworkNodeKindExtClient:
			topo.ExtClients = append(topo.ExtClients, vert)
		default:
			topo.Servers = append(topo.Servers, vert)
		}
	}
	names := make([]string, 0, len(byUser))
	for name := range byUser {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		devices := byUser[name]
		sortTopologyNodes(devices)
		topo.Users = append(topo.Users, models.NetworkTopologyUser{Username: name, Devices: devices})
	}
	sortTopologyNodes(topo.Servers)
	sortTopologyNodes(topo.ExtClients)
	return topo
}

func sortTopologyNodes(nodes []models.NetworkTopologyNode) {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
}

func (b *networkStatusBuilder) topologyNode(node *models.Node) models.NetworkTopologyNode {
	id := graphNodeID(node)
	vert := models.NetworkTopologyNode{
		ID:     id,
		Kind:   networkNodeKind(node),
		Name:   b.names[id],
		Status: node.Status,
	}
	if vert.Kind == models.NetworkNodeKindUser {
		vert.OwnerID = NodeOwnerUsername(node)
	}
	if node.IsStatic {
		vert.Address = node.StaticNode.Address
		vert.Connected = node.StaticNode.Enabled
		vert.GatewayNodeID = node.StaticNode.IngressGatewayID
		_, exitRouter := b.extClientExit(node)
		vert.InternetGatewayNodeID = exitRouter
		return vert
	}
	if node.HostID != uuid.Nil {
		vert.HostID = node.HostID.String()
	}
	vert.Address = ipNetAddr(node.Address)
	vert.Connected = node.Connected
	if node.IsRelayed {
		vert.GatewayNodeID = node.RelayedBy
	}
	_, exitRouter := b.nodeExit(node)
	vert.InternetGatewayNodeID = exitRouter
	if vert.Kind == models.NetworkNodeKindNode {
		vert.Roles = serverGatewayRole(node)
		vert.Egresses = b.topologyEgresses(node)
	}
	return vert
}

// serverGatewayRole collapses ingress, relay, and auto-relay into gateway.
func serverGatewayRole(node *models.Node) []string {
	if node.IsGw || node.IsIngressGateway || node.IsRelay || node.IsAutoRelay {
		return []string{models.NetworkServerRoleGateway}
	}
	return nil
}

// topologyEgresses lists active egresses this server routes. An internet egress
// marks the routing node as an exit; other types mark it as an egress.
// node.IsInternetGateway is not consulted.
func (b *networkStatusBuilder) topologyEgresses(node *models.Node) []models.NetworkTopologyEgress {
	id := node.ID.String()
	var out []models.NetworkTopologyEgress
	for i := range b.egresses {
		e := &b.egresses[i]
		if !routedBy(b.routers[e.ID], id) {
			continue
		}
		role := models.NetworkEgressRoleEgress
		if IsEgressInternetGateway(*e) {
			role = models.NetworkEgressRoleInternetGateway
		}
		out = append(out, models.NetworkTopologyEgress{
			ID:   e.ID,
			Name: e.Name,
			Type: string(e.Type),
			Role: role,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (b *networkStatusBuilder) nodeConnections(nodes []models.Node, nodeID string) (*models.NodeConnections, error) {
	var self *models.Node
	for i := range nodes {
		if graphNodeID(&nodes[i]) == nodeID {
			self = &nodes[i]
			break
		}
	}
	if self == nil {
		return nil, ErrTopologyNodeNotFound
	}
	conns := make([]models.NetworkConnection, 0)
	for i := range nodes {
		peer := &nodes[i]
		if graphNodeID(peer) == nodeID || graphNodeID(peer) == "" {
			continue
		}
		if !b.connectionAllowed(self, peer) {
			continue
		}
		conns = append(conns, b.connection(self, peer))
	}
	sort.Slice(conns, func(i, j int) bool { return conns[i].PeerID < conns[j].PeerID })
	return &models.NodeConnections{NodeID: nodeID, Connections: conns}, nil
}

// connectionAllowed reports whether the clicked node keeps a line to peer.
// User devices never connect to each other. Otherwise the line follows PeerAllowed.
func (b *networkStatusBuilder) connectionAllowed(node, peer *models.Node) bool {
	if networkNodeKind(node) == models.NetworkNodeKindUser && networkNodeKind(peer) == models.NetworkNodeKindUser {
		return false
	}
	if UserDevicesAreNotPeers(b.hostOf(node), b.hostOf(peer)) {
		return false
	}
	return PeerAllowed(b.ctx, *node, *peer, b.deviceDefault)
}

func (b *networkStatusBuilder) hostOf(node *models.Node) *schema.Host {
	if node == nil || node.IsStatic {
		return nil
	}
	host, ok := b.hosts[node.HostID.String()]
	if !ok {
		return nil
	}
	return &host
}

func (b *networkStatusBuilder) connection(node, peer *models.Node) models.NetworkConnection {
	peerID := graphNodeID(peer)
	conn := models.NetworkConnection{
		PeerID: peerID,
		Name:   b.names[peerID],
		Kind:   networkNodeKind(peer),
	}
	var via string
	conn.Type, via = b.connectionPath(node, peer)
	if via != "" {
		conn.Via = b.nodeRef(via)
	}
	b.applyConnectionLiveness(&conn, node, peer, via)
	return conn
}

// connectionPath classifies the clicked node's path to peer: direct, through a
// relay, or through an internet-egress routing node.
func (b *networkStatusBuilder) connectionPath(node, peer *models.Node) (models.NetworkConnectionType, string) {
	if node.IsStatic {
		return b.extClientConnectionPath(node, peer)
	}
	if peer.IsStatic {
		return b.pathToExtClient(node, peer)
	}
	return b.classifyRelay(b.relayBetween(node, peer))
}

func (b *networkStatusBuilder) relayBetween(node, peer *models.Node) string {
	exit, _ := b.nodeExit(node)
	bypass := exit != nil && InternetEgressBypassesEgressRoutes(*exit)
	return peerRelayNodeID(node, peer, b.peerIgnoreRelayedBy(node, peer, bypass))
}

func (b *networkStatusBuilder) classifyRelay(relayID string) (models.NetworkConnectionType, string) {
	if relayID == "" {
		return models.NetworkConnectionP2P, ""
	}
	if _, ok := b.inetRouters[relayID]; ok {
		return models.NetworkConnectionExit, relayID
	}
	return models.NetworkConnectionRelayed, relayID
}

// extClientConnectionPath is the extclient's view: its ingress is direct, and
// every other allowed peer is reached through that ingress or through the exit
// the ingress routes.
func (b *networkStatusBuilder) extClientConnectionPath(node, peer *models.Node) (models.NetworkConnectionType, string) {
	gwID := node.StaticNode.IngressGatewayID
	if gwID != "" && graphNodeID(peer) == gwID {
		return models.NetworkConnectionP2P, ""
	}
	exit, exitRouter := b.extClientExit(node)
	bypass := exit != nil && InternetEgressBypassesEgressRoutes(*exit)
	if exitRouter != "" && !(bypass && b.peerIsBypassDirectEgress(node, peer)) {
		return models.NetworkConnectionExit, exitRouter
	}
	if gwID != "" {
		return models.NetworkConnectionRelayed, gwID
	}
	return models.NetworkConnectionP2P, ""
}

// pathToExtClient is a real node's view of an extclient. The ingress gateway
// peers with it directly. Everyone else reaches it through that ingress, or
// through the viewer's relay or exit when that is how they reach the ingress.
func (b *networkStatusBuilder) pathToExtClient(node, peer *models.Node) (models.NetworkConnectionType, string) {
	gwID := peer.StaticNode.IngressGatewayID
	if gwID == "" {
		return models.NetworkConnectionP2P, ""
	}
	if node.ID.String() == gwID {
		return models.NetworkConnectionP2P, ""
	}
	if gw := b.nodesByID[gwID]; gw != nil && !gw.IsStatic {
		if relayID := b.relayBetween(node, gw); relayID != "" {
			return b.classifyRelay(relayID)
		}
	}
	return models.NetworkConnectionRelayed, gwID
}

func (b *networkStatusBuilder) applyConnectionLiveness(conn *models.NetworkConnection, node, peer *models.Node, via string) {
	if conn.Type == models.NetworkConnectionP2P {
		metric, ok := b.endpointMetric(node, graphNodeID(peer))
		if !ok {
			return
		}
		conn.Connected = metric.Connected
		conn.LatencyMs = metric.Latency
		conn.PercentUp = metric.PercentUp
		return
	}
	toVia, hasToVia := b.endpointMetric(node, via)
	viaNode := b.nodesByID[via]
	var hop models.Metric
	var hasHop bool
	if viaNode != nil {
		hop, hasHop = b.endpointMetric(viaNode, graphNodeID(peer))
	}
	if !hasToVia || !hasHop {
		return
	}
	conn.Connected = toVia.Connected && hop.Connected
	conn.LatencyMs = toVia.Latency + hop.Latency
	conn.PercentUp = min(toVia.PercentUp, hop.PercentUp)
}

// endpointMetric is the clicked node's measured link to targetID. An extclient
// has no metrics of its own; its only measured hop is the ingress gateway's
// record for the client.
func (b *networkStatusBuilder) endpointMetric(from *models.Node, targetID string) (models.Metric, bool) {
	if from == nil || targetID == "" {
		return models.Metric{}, false
	}
	if from.IsStatic {
		gwID := from.StaticNode.IngressGatewayID
		if targetID != gwID {
			return models.Metric{}, false
		}
		metric, ok := b.connectivity(gwID)[from.StaticNode.ClientID]
		return metric, ok
	}
	metric, ok := b.connectivity(graphNodeID(from))[targetID]
	return metric, ok
}
