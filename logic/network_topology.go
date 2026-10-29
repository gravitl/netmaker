package logic

import (
	"context"
	"errors"
	"maps"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// userPeerKey identifies one owner-to-peer user-policy decision.
type userPeerKey struct {
	owner  string
	peerID string
}

// userPolicyAccess is the set of peer IDs, client IDs, and tags an owner may reach.
type userPolicyAccess struct {
	allowAll bool
	dst      map[string]struct{}
}

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
	loc, country := b.nodeLocation(node)
	vert := models.NetworkTopologyNode{
		ID:       id,
		Kind:     networkNodeKind(node),
		Name:     b.names[id],
		Location: loc,
		Country:  country,
		Status:   node.Status,
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
		vert.IsGw = connectionIsGw(node)
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

// nodeLocation returns lat,lon and country for a node or static extclient.
func (b *networkStatusBuilder) nodeLocation(node *models.Node) (location, country string) {
	if node == nil {
		return "", ""
	}
	if node.IsStatic {
		return node.StaticNode.Location, node.StaticNode.Country
	}
	if host := b.hostOf(node); host != nil {
		return host.Location, host.CountryCode
	}
	return node.Location, node.CountryCode
}

// connectionIsGw reports whether a server node is a mesh/ingress/relay gateway.
func connectionIsGw(node *models.Node) bool {
	if node == nil || node.IsStatic || networkNodeKind(node) != models.NetworkNodeKindNode {
		return false
	}
	return len(serverGatewayRole(node)) > 0
}

// connectionEgresses returns routed egresses for a server node. User devices
// and extclients get none.
func (b *networkStatusBuilder) connectionEgresses(node *models.Node) []models.NetworkTopologyEgress {
	if node == nil || node.IsStatic || networkNodeKind(node) != models.NetworkNodeKindNode {
		return nil
	}
	return b.topologyEgresses(node)
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
	loc, country := b.nodeLocation(self)
	return &models.NodeConnections{
		NodeID:      nodeID,
		Name:        b.names[nodeID],
		Kind:        networkNodeKind(self),
		Location:    loc,
		Country:     country,
		IsGw:        connectionIsGw(self),
		Egresses:    b.connectionEgresses(self),
		Connections: conns,
	}, nil
}

// connectionAllowed reports whether the clicked node keeps a line to peer.
// User devices never connect to each other. A RAC extclient follows its owner's
// user policy, the same gate as extclient peering. Otherwise the line follows PeerAllowed.
func (b *networkStatusBuilder) connectionAllowed(node, peer *models.Node) bool {
	if networkNodeKind(node) == models.NetworkNodeKindUser && networkNodeKind(peer) == models.NetworkNodeKindUser {
		return false
	}
	if UserDevicesAreNotPeers(b.hostOf(node), b.hostOf(peer)) {
		return false
	}
	if node.IsStatic && node.IsUserNode {
		return b.userMayReach(node.StaticNode.OwnerID, peer)
	}
	if peer.IsStatic && peer.IsUserNode {
		return b.userMayReach(peer.StaticNode.OwnerID, node)
	}
	return b.peerAllowed(node, peer)
}

// peerAllowed is PeerAllowed with user-policy decisions memoized on the builder.
func (b *networkStatusBuilder) peerAllowed(node, peer *models.Node) bool {
	if skipResourcePolicyForUserDevices(node, peer) {
		return b.allowedViaUserOwnership(node, peer)
	}
	if b.deviceDefault {
		return true
	}
	return IsPeerAllowed(b.ctx, *node, *peer, false) || b.allowedViaUserOwnership(node, peer)
}

// allowedViaUserOwnership mirrors isAllowedViaUserOwnership using the builder cache
// and nodes already loaded for this network.
func (b *networkStatusBuilder) allowedViaUserOwnership(node, peer *models.Node) bool {
	if owner := NodeOwnerUsername(node); owner != "" {
		if b.userMayReach(owner, peer) || b.userAllowedViaRelay(owner, peer) {
			return true
		}
	}
	if owner := NodeOwnerUsername(peer); owner != "" {
		if b.userMayReach(owner, node) || b.userAllowedViaRelay(owner, node) {
			return true
		}
	}
	return false
}

// userMayReach reports whether owner may communicate with peer. Repeated pairs
// reuse the memo. When this builder loaded the network's user policies, the
// decision uses that data instead of another policy query per pair.
func (b *networkStatusBuilder) userMayReach(owner string, peer *models.Node) bool {
	if owner == "" || peer == nil {
		return false
	}
	peerID := graphNodeID(peer)
	if peerID == "" {
		return false
	}
	if b.userPeerAllowed == nil {
		b.userPeerAllowed = make(map[userPeerKey]bool)
	}
	key := userPeerKey{owner: owner, peerID: peerID}
	if allowed, ok := b.userPeerAllowed[key]; ok {
		return allowed
	}
	allowed := b.evalUserMayReach(owner, peer)
	if !allowed && b.userPoliciesLoaded && peer.IsIngressGateway {
		allowed = b.userMayReachExtClientOnIngress(owner, peer)
	}
	b.userPeerAllowed[key] = allowed
	return allowed
}

func (b *networkStatusBuilder) evalUserMayReach(owner string, peer *models.Node) bool {
	if !b.userPoliciesLoaded {
		ok, _ := IsUserAllowedToCommunicate(b.ctx, owner, *peer)
		return ok
	}
	if b.userDefault {
		return true
	}
	access := b.ownerAccess(owner)
	if access.allowAll {
		return true
	}
	if _, ok := access.dst[peer.ID.String()]; ok {
		return true
	}
	if _, ok := access.dst[graphNodeID(peer)]; ok {
		return true
	}
	for _, tagID := range peerTagIDs(peer) {
		if _, ok := access.dst[tagID]; ok {
			return true
		}
	}
	return false
}

// userMayReachExtClientOnIngress allows an ingress when the owner may reach any
// enabled non-RAC extclient attached to it. Extclients are already on the builder.
func (b *networkStatusBuilder) userMayReachExtClientOnIngress(owner string, ingress *models.Node) bool {
	ingressID := ingress.ID.String()
	for _, node := range b.nodesByID {
		if node == nil || !node.IsStatic || !node.StaticNode.Enabled ||
			node.StaticNode.IngressGatewayID != ingressID || node.StaticNode.RemoteAccessClientID != "" {
			continue
		}
		if b.userMayReach(owner, node) {
			return true
		}
	}
	return false
}

func (b *networkStatusBuilder) ownerAccess(owner string) userPolicyAccess {
	if b.userAccess == nil {
		b.userAccess = make(map[string]userPolicyAccess)
	}
	if access, ok := b.userAccess[owner]; ok {
		return access
	}
	access := b.buildOwnerAccess(owner)
	b.userAccess[owner] = access
	return access
}

func (b *networkStatusBuilder) buildOwnerAccess(owner string) userPolicyAccess {
	access := userPolicyAccess{dst: map[string]struct{}{}}
	user := b.user(owner)
	if user == nil {
		return access
	}
	groups := userPolicyGroupIDs(user, b.network)
	for _, policy := range b.userPolicies {
		if !policy.Enabled || policy.RuleType != models.UserPolicy || !policyAppliesToUser(policy, owner, groups) {
			continue
		}
		dst := ConvAclTagToValueMap(policy.Dst)
		for _, tag := range policy.Dst {
			if tag.ID != models.EgressID {
				continue
			}
			egress, ok := b.egressByID(tag.Value)
			if !ok || !egress.Status {
				continue
			}
			for nodeID := range egress.Nodes {
				dst[nodeID] = struct{}{}
			}
			for tagID := range egress.Tags {
				dst[tagID] = struct{}{}
			}
		}
		if _, ok := dst["*"]; ok {
			access.allowAll = true
		}
		for id := range dst {
			access.dst[id] = struct{}{}
		}
	}
	return access
}

func (b *networkStatusBuilder) egressByID(id string) (schema.Egress, bool) {
	for i := range b.eli {
		if b.eli[i].ID == id {
			return b.eli[i], true
		}
	}
	return schema.Egress{}, false
}

func policyAppliesToUser(policy models.Acl, username string, groups map[schema.UserGroupID]struct{}) bool {
	src := ConvAclTagToValueMap(policy.Src)
	if _, ok := src[username]; ok {
		return true
	}
	for groupID := range groups {
		if _, ok := src[groupID.String()]; ok {
			return true
		}
	}
	return false
}

// userPolicyGroupIDs includes network default groups implied by global membership,
// matching listPoliciesOfUser.
func userPolicyGroupIDs(user *schema.User, network string) map[schema.UserGroupID]struct{} {
	groups := map[schema.UserGroupID]struct{}{}
	if user != nil {
		for id := range user.UserGroups.Data() {
			groups[id] = struct{}{}
		}
	}
	if network == "" {
		return groups
	}
	if _, ok := groups[userGroupID("global", schema.NetworkAdmin)]; ok {
		groups[userGroupID(network, schema.NetworkAdmin)] = struct{}{}
	}
	if _, ok := groups[userGroupID("global", schema.NetworkUser)]; ok {
		groups[userGroupID(network, schema.NetworkUser)] = struct{}{}
	}
	return groups
}

func userGroupID(prefix string, role schema.UserRoleID) schema.UserGroupID {
	return schema.UserGroupID(prefix + "-" + string(role) + "-grp")
}

func peerTagIDs(node *models.Node) []string {
	var tags map[models.TagID]struct{}
	if node.Mutex != nil {
		node.Mutex.Lock()
		tags = maps.Clone(node.Tags)
		node.Mutex.Unlock()
	} else {
		tags = node.Tags
	}
	ids := make([]string, 0, len(tags))
	for tagID := range tags {
		ids = append(ids, tagID.String())
	}
	return ids
}

// userAllowedViaRelay reports whether owner may reach any node peer relays.
// Relayed nodes are taken from the builder; a hand-built builder falls back to getNodeByID.
func (b *networkStatusBuilder) userAllowedViaRelay(owner string, peer *models.Node) bool {
	if owner == "" || peer == nil {
		return false
	}
	relayedIDs := peer.RelayedNodes
	if len(relayedIDs) == 0 && len(peer.InetNodeReq.InetNodeClientIDs) > 0 {
		relayedIDs = peer.InetNodeReq.InetNodeClientIDs
	}
	if len(relayedIDs) == 0 && !peer.IsRelay {
		return false
	}
	for _, id := range relayedIDs {
		if id == "" {
			continue
		}
		relayed := b.nodesByID[id]
		if relayed == nil {
			if b.userPoliciesLoaded {
				continue
			}
			node, err := getNodeByID(id)
			if err != nil {
				continue
			}
			relayed = &node
		}
		if b.userMayReach(owner, relayed) {
			return true
		}
	}
	return false
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
	loc, country := b.nodeLocation(peer)
	conn := models.NetworkConnection{
		PeerID:   peerID,
		Name:     b.names[peerID],
		Kind:     networkNodeKind(peer),
		IsGw:     connectionIsGw(peer),
		Egresses: b.connectionEgresses(peer),
		Location: loc,
		Country:  country,
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
// relay, or through the viewer's selected internet exit.
func (b *networkStatusBuilder) connectionPath(node, peer *models.Node) (models.NetworkConnectionType, string) {
	if node.IsStatic {
		return b.extClientConnectionPath(node, peer)
	}
	if peer.IsStatic {
		return b.pathToExtClient(node, peer)
	}
	return b.meshConnectionPath(node, peer)
}

// meshConnectionPath classifies a real-node path. "exit" is only for traffic
// hairpinning through the viewer's selected internet-egress router. A peer (or
// the viewer) that is RelayedBy / auto-relayed through a node that also happens
// to route internet egress is still "relayed" — being an exit router alone does
// not make every path through it an exit path.
func (b *networkStatusBuilder) meshConnectionPath(node, peer *models.Node) (models.NetworkConnectionType, string) {
	if peer == nil {
		return models.NetworkConnectionP2P, ""
	}
	exit, exitRouter := b.nodeExit(node)
	bypass := exit != nil && InternetEgressBypassesEgressRoutes(*exit)
	ignore := b.peerIgnoreRelayedBy(node, peer, bypass)
	peerID := peer.ID.String()

	if relayID, ok := node.AutoRelayedPeers[peerID]; ok && relayID != "" {
		if exitRouter != "" && relayID == exitRouter {
			return models.NetworkConnectionExit, relayID
		}
		return models.NetworkConnectionRelayed, relayID
	}
	if peer.IsRelayed && peer.RelayedBy != "" && peer.RelayedBy != node.ID.String() && peer.RelayedBy != ignore {
		return models.NetworkConnectionRelayed, peer.RelayedBy
	}
	if node.IsRelayed && node.RelayedBy != "" && node.RelayedBy != peerID && node.RelayedBy != ignore {
		if exitRouter != "" && node.RelayedBy == exitRouter {
			return models.NetworkConnectionExit, node.RelayedBy
		}
		return models.NetworkConnectionRelayed, node.RelayedBy
	}
	return models.NetworkConnectionP2P, ""
}

// extClientConnectionPath is the extclient's view: its gateway is direct, and
// every other allowed peer is reached through that gateway or through the exit
// the gateway routes.
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

// pathToExtClient is a real node's view of an extclient. Ingress and relay are
// the same gateway: that gateway peers with the extclient, and it does not use
// an exit node. Everyone else reaches the extclient through that gateway.
func (b *networkStatusBuilder) pathToExtClient(node, peer *models.Node) (models.NetworkConnectionType, string) {
	gwID := peer.StaticNode.IngressGatewayID
	if gwID == "" || node.ID.String() == gwID {
		return models.NetworkConnectionP2P, ""
	}
	return models.NetworkConnectionRelayed, gwID
}

func (b *networkStatusBuilder) applyConnectionLiveness(conn *models.NetworkConnection, node, peer *models.Node, via string) {
	if peer != nil && peer.IsStatic {
		applyHopLiveness(conn, b.extClientHops(node, peer, via))
		return
	}
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

// extClientHops are the measured hops from viewer to a static extclient.
// The gateway is the only via: viewer→gateway from the viewer's connectivity,
// then gateway→extclient from that gateway's connectivity map keyed by client
// ID. Missing hops are omitted.
func (b *networkStatusBuilder) extClientHops(viewer, peer *models.Node, via string) []models.Metric {
	if peer == nil {
		return nil
	}
	gwID := peer.StaticNode.IngressGatewayID
	var hops []models.Metric
	if gwID == "" {
		if metric, ok := b.endpointMetric(viewer, graphNodeID(peer)); ok {
			hops = append(hops, metric)
		}
		return hops
	}
	if via == gwID {
		if metric, ok := b.endpointMetric(viewer, gwID); ok {
			hops = append(hops, metric)
		}
	}
	if metric, ok := b.connectivity(gwID)[peer.StaticNode.ClientID]; ok {
		hops = append(hops, metric)
	}
	return hops
}

func applyHopLiveness(conn *models.NetworkConnection, hops []models.Metric) {
	if len(hops) == 0 || conn == nil {
		return
	}
	conn.Connected = hops[0].Connected
	conn.LatencyMs = hops[0].Latency
	conn.PercentUp = hops[0].PercentUp
	for _, hop := range hops[1:] {
		conn.Connected = conn.Connected && hop.Connected
		conn.LatencyMs += hop.Latency
		if hop.PercentUp < conn.PercentUp {
			conn.PercentUp = hop.PercentUp
		}
	}
}

// endpointMetric is the clicked node's measured link to targetID. An extclient
// has no metrics of its own; its only measured hop is its gateway's record for
// the client.
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
