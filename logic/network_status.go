package logic

import (
	"context"
	"encoding/json"
	"net"
	"sort"
	"time"

	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"golang.org/x/exp/slog"
)

// NetworkStatusOptions filters the network status.
type NetworkStatusOptions struct {
	// Kinds limits the entries to these kinds. Empty includes every kind.
	Kinds map[models.NetworkNodeKind]struct{}
	// IncludePeers and IncludeEgress control the per-peer and per-egress
	// details. Peer and egress counts are always filled.
	IncludePeers  bool
	IncludeEgress bool
}

// egressRouter is a routing node of an egress and its route metric.
type egressRouter struct {
	nodeID string
	metric uint32
}

// networkStatusBuilder holds the network-wide state shared by every entry.
type networkStatusBuilder struct {
	ctx         context.Context
	opts        NetworkStatusOptions
	eli         []schema.Egress
	egresses    []schema.Egress           // active egresses with routing nodes
	routers     map[string][]egressRouter // egress ID -> routers, lowest metric first
	inetRouters map[string]struct{}
	hosts       map[string]schema.Host
	names       map[string]string
	nodesByID   map[string]*models.Node

	acls          []models.Acl
	userPolicies  []models.Acl
	deviceDefault bool
	userDefault   bool
	users         map[string]*schema.User // nil value: lookup failed

	relayConnectivity map[string]map[string]models.Metric // relay node ID -> its links
}

// GetNetworkStatus returns the status of the nodes and extclients in the network.
func GetNetworkStatus(ctx context.Context, network string, opts NetworkStatusOptions) (*models.NetworkStatus, error) {
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
	eli, err := (&schema.Egress{Network: network}).ListByNetwork(db.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	b := &networkStatusBuilder{
		ctx:               ctx,
		opts:              opts,
		eli:               eli,
		inetRouters:       InternetEgressRoutingNodeIDsFromList(eli),
		hosts:             networkStatusHosts(ctx, nodes),
		names:             make(map[string]string, len(nodes)),
		nodesByID:         make(map[string]*models.Node, len(nodes)),
		userPolicies:      ListUserPolicies(ctx, schema.NetworkID(network)),
		users:             make(map[string]*schema.User),
		relayConnectivity: make(map[string]map[string]models.Metric),
	}
	b.acls, _ = ListAclsByNetwork(ctx, schema.NetworkID(network))
	if p, err := GetDefaultPolicy(ctx, schema.NetworkID(network), models.DevicePolicy); err == nil {
		b.deviceDefault = p.Enabled
	}
	b.userDefault = userDeviceEgressDefaultActive(ctx, network)

	var selected []models.Node
	for i := range nodes {
		node := &nodes[i]
		if node.IsStatic {
			b.names[node.StaticNode.ClientID] = node.StaticNode.ClientID
		} else {
			b.nodesByID[node.ID.String()] = node
			if host, ok := b.hosts[node.HostID.String()]; ok {
				b.names[node.ID.String()] = host.Name
			}
		}
		if _, ok := opts.Kinds[networkNodeKind(node)]; len(opts.Kinds) == 0 || ok {
			selected = append(selected, *node)
		}
	}
	b.indexEgressRouters()
	selected = AddStatusToNodes(ctx, selected, true)

	status := &models.NetworkStatus{
		Network:     network,
		GeneratedAt: time.Now().UTC().Unix(),
		Nodes:       make([]models.NetworkNodeStatus, 0, len(selected)),
	}
	for i := range selected {
		node := &selected[i]
		var nodeStatus models.NetworkNodeStatus
		if node.IsStatic {
			nodeStatus = b.extClientNetworkStatus(node)
		} else {
			nodeStatus = b.nodeNetworkStatus(node)
		}
		countNetworkNodeStatus(&status.Summary, nodeStatus.Status)
		status.Nodes = append(status.Nodes, nodeStatus)
	}
	return status, nil
}

func networkNodeKind(node *models.Node) models.NetworkNodeKind {
	if node.IsStatic {
		if node.IsUserNode {
			return models.NetworkNodeKindUser
		}
		return models.NetworkNodeKindExtClient
	}
	if IsUserOwnedDevice(node) {
		return models.NetworkNodeKindUser
	}
	return models.NetworkNodeKindNode
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

// indexEgressRouters resolves the routing nodes of every active egress,
// directly assigned or, except for internet egress, through tags.
func (b *networkStatusBuilder) indexEgressRouters() {
	b.routers = make(map[string][]egressRouter)
	for _, e := range b.eli {
		if !e.Status {
			continue
		}
		seen := make(map[string]struct{})
		var routers []egressRouter
		for nodeID, v := range e.Nodes {
			if _, ok := b.nodesByID[nodeID]; ok {
				seen[nodeID] = struct{}{}
				routers = append(routers, egressRouter{nodeID: nodeID, metric: egressRouteMetric(v)})
			}
		}
		if len(e.Tags) > 0 && !IsEgressInternetGateway(e) {
			for nodeID, node := range b.nodesByID {
				if _, ok := seen[nodeID]; ok {
					continue
				}
				for tagID := range node.Tags {
					if v, ok := e.Tags[tagID.String()]; ok {
						routers = append(routers, egressRouter{nodeID: nodeID, metric: egressRouteMetric(v)})
						break
					}
				}
			}
		}
		if len(routers) == 0 {
			continue
		}
		sort.Slice(routers, func(i, j int) bool {
			if routers[i].metric != routers[j].metric {
				return routers[i].metric < routers[j].metric
			}
			return routers[i].nodeID < routers[j].nodeID
		})
		b.routers[e.ID] = routers
		b.egresses = append(b.egresses, e)
	}
}

func egressRouteMetric(v interface{}) uint32 {
	switch m := v.(type) {
	case json.Number:
		if m64, err := m.Int64(); err == nil {
			return uint32(m64)
		}
	case float64:
		return uint32(m)
	case int:
		return uint32(m)
	}
	return 256
}

func (b *networkStatusBuilder) nodeNetworkStatus(node *models.Node) models.NetworkNodeStatus {
	nodeID := node.ID.String()
	host := b.hosts[node.HostID.String()]
	_, isInetRouter := b.inetRouters[nodeID]
	nodeStatus := models.NetworkNodeStatus{
		ID:                nodeID,
		Kind:              networkNodeKind(node),
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
		IsEgress:          routesEgress(node, b.eli),
		Status:            node.Status,
		Connected:         node.Connected,
		LastCheckIn:       unixOrZero(node.LastCheckIn),
	}
	if host.EndpointIP != nil {
		nodeStatus.EndpointIP = host.EndpointIP.String()
	} else if host.EndpointIPv6 != nil {
		nodeStatus.EndpointIP = host.EndpointIPv6.String()
	}
	if node.IsRelayed {
		nodeStatus.GatewayNodeID = node.RelayedBy
	}
	exit, exitRouterID := b.nodeExit(node)
	nodeStatus.InternetGatewayNodeID = exitRouterID

	var connectivity map[string]models.Metric
	if metrics, err := GetMetrics(b.ctx, nodeID); err == nil && metrics != nil {
		nodeStatus.MetricsUpdatedAt = unixOrZero(metrics.UpdatedAt)
		connectivity = metrics.Connectivity
	}
	for peerID, metric := range connectivity {
		nodeStatus.TotalPeers++
		if metric.Connected {
			nodeStatus.ConnectedPeers++
		}
		if !b.opts.IncludePeers {
			continue
		}
		peerStatus := peerNetworkStatus(peerID, metric, b.names)
		if relayID := peerRelayNodeID(node, b.nodesByID[peerID]); relayID != "" {
			peerStatus.IsRelayed = true
			peerStatus.Via = b.nodeRef(relayID)
		}
		nodeStatus.Peers = append(nodeStatus.Peers, peerStatus)
	}

	// A bypassing exit keeps specific egress routers as direct peers, even
	// relayed or auto-relayed ones. Without bypass, an exit client is relayed by
	// the exit like any relayed node, so all its egress traffic hairpins there.
	bypass := exit != nil && InternetEgressBypassesEgressRoutes(*exit)
	for i := range b.egresses {
		e := &b.egresses[i]
		routers := b.routers[e.ID]
		inUseRouter, ok := b.egressApplies(e, exit, exitRouterID)
		if !ok || routedBy(routers, nodeID) || !b.nodeHasEgressAccess(node, e, routers) {
			continue
		}
		var path egressPath
		switch {
		case node.IsRelayed && node.RelayedBy != "" && (!bypass || IsEgressInternetGateway(*e)):
			toRelay, hasToRelay := connectivity[node.RelayedBy]
			path = relayedEgressPath(routers, inUseRouter, node.RelayedBy, toRelay, hasToRelay, b.connectivity(node.RelayedBy))
		default:
			path = directEgressPath(routers, inUseRouter, connectivity)
			if bypass {
				break
			}
			// The router itself may be relayed, or auto-relayed for this node.
			if relayID := peerRelayNodeID(node, b.nodesByID[path.routerID]); relayID != "" {
				toRelay, hasToRelay := connectivity[relayID]
				relayed := relayedEgressPath([]egressRouter{{nodeID: path.routerID}}, path.routerID, relayID,
					toRelay, hasToRelay, b.connectivity(relayID))
				relayed.connectedCnt = path.connectedCnt
				path = relayed
			}
		}
		egressStatus, ok := b.newEgressStatus(e, routers, path.routerID)
		if !ok {
			continue
		}
		b.applyEgressPath(&egressStatus, path)
		b.addEgress(&nodeStatus, egressStatus)
	}
	return nodeStatus
}

// extClientNetworkStatus reports an extclient's link to its ingress gateway,
// which is its only peer, and its egress paths through that gateway. Link
// metrics come from the gateway.
func (b *networkStatusBuilder) extClientNetworkStatus(node *models.Node) models.NetworkNodeStatus {
	ext := node.StaticNode
	gwID := ext.IngressGatewayID
	nodeStatus := models.NetworkNodeStatus{
		ID:            ext.ClientID,
		Kind:          networkNodeKind(node),
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
		GatewayNodeID: gwID,
	}
	exit, exitRouterID := b.extClientExit(node)
	nodeStatus.InternetGatewayNodeID = exitRouterID

	var gwConnectivity map[string]models.Metric
	var gwLink models.Metric
	var hasGwLink bool
	if gwID != "" {
		if metrics, err := GetMetrics(b.ctx, gwID); err == nil && metrics != nil {
			gwConnectivity = metrics.Connectivity
			if gwLink, hasGwLink = gwConnectivity[ext.ClientID]; hasGwLink {
				nodeStatus.MetricsUpdatedAt = unixOrZero(metrics.UpdatedAt)
			}
		}
	}
	if hasGwLink {
		nodeStatus.TotalPeers = 1
		if gwLink.Connected {
			nodeStatus.ConnectedPeers = 1
		}
		if b.opts.IncludePeers {
			peerStatus := peerNetworkStatus(gwID, gwLink, b.names)
			// The gateway records bytes from its own side; flip them to the extclient's.
			peerStatus.BytesSent, peerStatus.BytesReceived = gwLink.TotalReceived, gwLink.TotalSent
			nodeStatus.Peers = []models.NetworkPeerStatus{peerStatus}
		}
	}

	for i := range b.egresses {
		e := &b.egresses[i]
		inUseRouter, ok := b.egressApplies(e, exit, exitRouterID)
		if !ok || (!IsEgressInternetGateway(*e) && !b.extClientHasEgressAccess(node, e)) {
			continue
		}
		routers := b.routers[e.ID]
		path := relayedEgressPath(routers, inUseRouter, gwID, gwLink, hasGwLink, gwConnectivity)
		egressStatus, ok := b.newEgressStatus(e, routers, path.routerID)
		if !ok {
			continue
		}
		b.applyEgressPath(&egressStatus, path)
		b.addEgress(&nodeStatus, egressStatus)
	}
	return nodeStatus
}

// egressPath is how an entry reaches an egress: the router in use and the
// link to it, possibly through a relay.
type egressPath struct {
	routerID     string
	connectedCnt int
	relayID      string
	connected    bool
	latencyMs    int64
	percentUp    float64
}

// directEgressPath reaches the router in use over the entry's own link to it.
func directEgressPath(routers []egressRouter, preferred string, connectivity map[string]models.Metric) egressPath {
	var p egressPath
	p.routerID, p.connectedCnt = pickEgressRouter(routers, preferred, func(id string) bool {
		return connectivity[id].Connected
	})
	link := connectivity[p.routerID]
	p.connected, p.latencyMs, p.percentUp = link.Connected, link.Latency, link.PercentUp
	return p
}

// relayedEgressPath reaches the egress through relayID: the entry's link to
// the relay, then the relay's link to the router. Routers are seen from the
// relay, which reaches itself; when the relay routes the egress, the first
// hop is the whole path.
func relayedEgressPath(routers []egressRouter, preferred, relayID string, toRelay models.Metric, hasToRelay bool,
	relayConnectivity map[string]models.Metric) egressPath {
	var p egressPath
	p.routerID, p.connectedCnt = pickEgressRouter(routers, preferred, func(id string) bool {
		return id == relayID || relayConnectivity[id].Connected
	})
	p.connected = hasToRelay && toRelay.Connected
	p.latencyMs, p.percentUp = toRelay.Latency, toRelay.PercentUp
	if p.routerID != relayID {
		hop := relayConnectivity[p.routerID]
		p.relayID = relayID
		p.connected = p.connected && hop.Connected
		p.latencyMs += hop.Latency
		p.percentUp = min(p.percentUp, hop.PercentUp)
	}
	return p
}

func (b *networkStatusBuilder) applyEgressPath(egressStatus *models.NetworkEgressStatus, path egressPath) {
	egressStatus.RoutingNode = b.nodeRef(path.routerID)
	egressStatus.RoutingNodesConnected = path.connectedCnt
	egressStatus.Connected = path.connected
	egressStatus.LatencyMs = path.latencyMs
	egressStatus.PercentUp = path.percentUp
	if path.relayID != "" {
		egressStatus.IsRelayed = true
		egressStatus.Via = b.nodeRef(path.relayID)
	}
}

// nodeRef returns the ID and host name of a node, or nil for an empty ID.
func (b *networkStatusBuilder) nodeRef(nodeID string) *models.NetworkNodeRef {
	if nodeID == "" {
		return nil
	}
	return &models.NetworkNodeRef{ID: nodeID, Name: b.names[nodeID]}
}

// connectivity returns a node's reported links, cached per request.
func (b *networkStatusBuilder) connectivity(nodeID string) map[string]models.Metric {
	if c, ok := b.relayConnectivity[nodeID]; ok {
		return c
	}
	var c map[string]models.Metric
	if metrics, err := GetMetrics(b.ctx, nodeID); err == nil && metrics != nil {
		c = metrics.Connectivity
	}
	b.relayConnectivity[nodeID] = c
	return c
}

func (b *networkStatusBuilder) addEgress(nodeStatus *models.NetworkNodeStatus, egressStatus models.NetworkEgressStatus) {
	nodeStatus.TotalEgresses++
	if egressStatus.Connected {
		nodeStatus.ConnectedEgresses++
	}
	if b.opts.IncludeEgress {
		nodeStatus.Egresses = append(nodeStatus.Egresses, egressStatus)
	}
}

// nodeHasEgressAccess mirrors the access check used when building peer updates.
func (b *networkStatusBuilder) nodeHasEgressAccess(node *models.Node, e *schema.Egress, routers []egressRouter) bool {
	if IsUserOwnedDevice(node) {
		if b.userDefault {
			return true
		}
		user := b.user(NodeOwnerUsername(node))
		return user != nil && DoesUserHaveAccessToEgress(user, e, b.userPolicies)
	}
	if b.deviceDefault || NodeHasEgressAccess(b.ctx, node, e, b.acls) {
		return true
	}
	for _, r := range routers {
		if doesNodeHaveAccessToEgressByRoutingPolicy(node, b.nodesByID[r.nodeID], e, b.acls) {
			return true
		}
	}
	return false
}

// extClientHasEgressAccess mirrors GetEgressRangesOnNetwork: user extclients
// follow their owner's user policies, other extclients get every egress.
func (b *networkStatusBuilder) extClientHasEgressAccess(node *models.Node, e *schema.Egress) bool {
	if b.userDefault || !node.IsUserNode || node.StaticNode.OwnerID == "" {
		return true
	}
	user := b.user(node.StaticNode.OwnerID)
	return user != nil && DoesUserHaveAccessToEgress(user, e, b.userPolicies)
}

func (b *networkStatusBuilder) user(username string) *schema.User {
	if user, ok := b.users[username]; ok {
		return user
	}
	user := &schema.User{Username: username}
	if err := user.GetWithMembership(b.ctx); err != nil {
		user = nil
	}
	b.users[username] = user
	return user
}

// nodeExit returns the internet egress the node routes all traffic through and
// its routing node. The node must be an IGW client of a routing node of its
// assigned egress (InternetGwID, derived from RelayedBy) and be allowed to use
// it. A selection that failed open (exit down) or that ACLs suppress does not
// route traffic.
func (b *networkStatusBuilder) nodeExit(node *models.Node) (*schema.Egress, string) {
	e := assignedInternetEgress(node, b.eli)
	if e == nil || node.InternetGwID == "" {
		return nil, ""
	}
	if _, ok := e.Nodes[node.InternetGwID]; !ok {
		return nil, ""
	}
	if !b.nodeHasEgressAccess(node, e, b.routers[e.ID]) {
		return nil, ""
	}
	return e, node.InternetGwID
}

// extClientExit mirrors ExtClientUsesInternetEgress: an extclient routes all
// traffic only through an internet egress that its own gateway routes.
func (b *networkStatusBuilder) extClientExit(node *models.Node) (*schema.Egress, string) {
	gwID := node.StaticNode.IngressGatewayID
	e := assignedInternetEgress(node, b.eli)
	if e == nil || gwID == "" {
		return nil, ""
	}
	if _, ok := e.Nodes[gwID]; !ok {
		return nil, ""
	}
	return e, gwID
}

// egressApplies reports whether egress e routes the entry's traffic. An
// internet egress applies only when it is the entry's selected exit, and then
// the router in use is the exit routing node.
func (b *networkStatusBuilder) egressApplies(e, exit *schema.Egress, exitRouterID string) (inUseRouter string, ok bool) {
	if !IsEgressInternetGateway(*e) {
		return "", true
	}
	if exit == nil || exit.ID != e.ID {
		return "", false
	}
	return exitRouterID, true
}

// newEgressStatus fills the egress fields of an egress status. It reports
// false when the egress currently routes nothing (e.g. unresolved domains).
func (b *networkStatusBuilder) newEgressStatus(e *schema.Egress, routers []egressRouter, routerID string) (models.NetworkEgressStatus, bool) {
	isInternet := IsEgressInternetGateway(*e)
	// ::/0 is only routed through exits with a public IPv6 endpoint.
	includeIPv6 := !isInternet
	if router, ok := b.nodesByID[routerID]; ok && isInternet {
		includeIPv6 = len(b.hosts[router.HostID.String()].EndpointIPv6) > 0
	}
	ranges := ExpandEgressRouteRanges(*e, includeIPv6)
	if len(ranges) == 0 {
		return models.NetworkEgressStatus{}, false
	}
	return models.NetworkEgressStatus{
		EgressID:          e.ID,
		Name:              e.Name,
		Ranges:            ranges,
		Domains:           ConfiguredDomainsForEgress(*e),
		IsInternet:        isInternet,
		RoutingNodesTotal: len(routers),
	}, true
}

// pickEgressRouter returns the router in use and the connected router count.
// The router in use is preferred when set (an exit pins its routing node),
// otherwise the lowest-metric connected router, otherwise the lowest-metric
// router. routers must be sorted by metric.
func pickEgressRouter(routers []egressRouter, preferred string, connected func(nodeID string) bool) (string, int) {
	chosen := preferred
	connectedCnt := 0
	for _, r := range routers {
		if !connected(r.nodeID) {
			continue
		}
		connectedCnt++
		if chosen == "" {
			chosen = r.nodeID
		}
	}
	if chosen == "" && len(routers) > 0 {
		chosen = routers[0].nodeID
	}
	return chosen, connectedCnt
}

func routedBy(routers []egressRouter, nodeID string) bool {
	for _, r := range routers {
		if r.nodeID == nodeID {
			return true
		}
	}
	return false
}

func peerNetworkStatus(peerID string, metric models.Metric, names map[string]string) models.NetworkPeerStatus {
	name := names[peerID]
	if name == "" {
		name = metric.NodeName
	}
	return models.NetworkPeerStatus{
		PeerID:        peerID,
		Name:          name,
		Connected:     metric.Connected,
		LatencyMs:     metric.Latency,
		PercentUp:     metric.PercentUp,
		BytesSent:     metric.TotalSent,
		BytesReceived: metric.TotalReceived,
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
