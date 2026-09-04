package logic

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var (
	DeleteNodesCh = make(chan *models.Node, 100)
)

// GetNetworkNodes - gets the nodes of a network
func GetNetworkNodes(ctx context.Context, network string) ([]models.Node, error) {
	allnodes, err := GetAllNodes(ctx)
	if err != nil {
		return []models.Node{}, err
	}

	return GetNetworkNodesMemory(allnodes, network), nil
}

// GetHostNodes - fetches all nodes part of the host
func GetHostNodes(host *schema.Host) []models.Node {
	nodes := []models.Node{}
	for _, nodeID := range host.Nodes {
		node, err := GetNodeByID(nodeID)
		if err == nil {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// GetNetworkNodesMemory - gets all nodes belonging to a network from list in memory
func GetNetworkNodesMemory(allNodes []models.Node, network string) []models.Node {
	var nodes = make([]models.Node, 0, len(allNodes))
	for i := range allNodes {
		node := allNodes[i]
		if node.Network == network {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

var (
	pendingCheckins   = make(map[string]time.Time)
	pendingCheckinsMu sync.Mutex
)

// UpdateNodeCheckin - buffers the checkin timestamp in memory when caching is enabled.
// The actual DB write is deferred to FlushNodeCheckins (every 30s).
// When caching is disabled (HA mode), writes directly to the DB.
func UpdateNodeCheckin(nodeID string) error {
	if servercfg.CacheEnabled() {
		pendingCheckinsMu.Lock()
		pendingCheckins[nodeID] = time.Now().UTC()
		pendingCheckinsMu.Unlock()
		return nil
	}

	node := &schema.Node{
		ID:          nodeID,
		LastCheckIn: time.Now().UTC(),
	}
	return node.UpdateLastCheckIn(db.WithContext(context.TODO()))
}

// FlushNodeCheckins - writes all buffered check-in updates to the DB in one batch.
// Called periodically (e.g., every 30s) to avoid per-checkin write lock contention.
// Uses a single transaction so SQLite (MaxOpenConns=1) holds the write lock once
// instead of once per node, which previously made /api/server/status Ping time out.
func FlushNodeCheckins() {
	pendingCheckinsMu.Lock()
	batch := pendingCheckins
	pendingCheckins = make(map[string]time.Time)
	pendingCheckinsMu.Unlock()
	if len(batch) == 0 {
		return
	}

	requeue := func() {
		pendingCheckinsMu.Lock()
		for id, checkin := range batch {
			if existing, exists := pendingCheckins[id]; !exists || checkin.After(existing) {
				pendingCheckins[id] = checkin
			}
		}
		pendingCheckinsMu.Unlock()
	}

	ctx := db.WithContext(context.TODO())
	tx := db.FromContext(ctx).Begin()
	if tx.Error != nil {
		slog.Error("FlushNodeCheckins: failed to begin transaction", "error", tx.Error, "total", len(batch))
		requeue()
		return
	}

	for id, checkin := range batch {
		if err := tx.Model(&schema.Node{}).Where("id = ?", id).Update("last_check_in", checkin).Error; err != nil {
			_ = tx.Rollback()
			slog.Error("FlushNodeCheckins: failed to persist checkins", "error", err, "total", len(batch))
			requeue()
			return
		}
	}
	if err := tx.Commit().Error; err != nil {
		slog.Error("FlushNodeCheckins: failed to commit checkins", "error", err, "total", len(batch))
		requeue()
	}
}

// UpsertNode - updates node in the DB
func UpsertNode(newNode *models.Node) error {
	_node := ConvertModelsNodeToSchemaNode(newNode)
	if _node.ID == "" {
		return errors.New("error converting models.Node to schema.Node")
	}
	return _node.Upsert(db.WithContext(context.TODO()))
}

// UpdateNode - takes a node and updates another node with it's values
func UpdateNode(currentNode *models.Node, newNode *models.Node) error {
	if newNode.Address.IP.String() != currentNode.Address.IP.String() {
		ctx := scope.WithContext(db.WithContext(context.TODO()), scope.TenantScope, currentNode.TenantID)
		network := &schema.Network{Name: newNode.Network}
		if err := network.Get(ctx); err == nil {
			if !IsAddressInCIDR(newNode.Address.IP, network.AddressRange) {
				return fmt.Errorf("invalid address provided; out of network range for node %s", newNode.ID)
			}
		}
	}
	newNode.Fill(currentNode, servercfg.IsPro)

	if newNode.ID == currentNode.ID {
		if !currentNode.Connected && newNode.Connected {
			newNode.SetLastCheckIn()
			newNode.Status = schema.OnlineSt
			//go TriggerCollectMetrics(newNode.HostID.String(), newNode.ID.String(), "reconnect")
		}
		if currentNode.Connected && !newNode.Connected {
			newNode.Status = schema.Disconnected
		}

		_node := ConvertModelsNodeToSchemaNode(newNode)
		if _node.ID == "" {
			return errors.New("error converting models.Node to schema.Node")
		}
		return _node.Upsert(db.WithContext(context.TODO()))
	}

	return fmt.Errorf("failed to update node %s, cannot change ID", currentNode.ID.String())
}

// DeleteNode - marks node for deletion (and adds to zombie list) if called by UI or deletes node if called by node
// cleanupNodeReferences handles best-effort cleanup of all external references
// to a node (relay, internet gw, failover, nameservers, ACL, egress, enrollment keys).
// Errors are logged but do not prevent node deletion.
func cleanupNodeReferences(ctx context.Context, node *models.Node) {
	if node.IsIngressGateway {
		if err := DeleteGatewayExtClients(ctx, node.ID.String(), node.Network); err != nil {
			slog.Error("failed to delete ext clients", "nodeid", node.ID.String(), "error", err.Error())
		}
	}
	if node.IsRelayed {
		relayNode, err := GetNodeByID(node.RelayedBy)
		if err == nil {
			relayedNodes := []string{}
			for _, relayedNodeID := range relayNode.RelayedNodes {
				if relayedNodeID == node.ID.String() {
					continue
				}
				relayedNodes = append(relayedNodes, relayedNodeID)
			}
			relayNode.RelayedNodes = relayedNodes
			UpsertNode(&relayNode)
		}
	}
	// Always scrub this node ID from every gateway RelayedClients map in the
	// network. RelayedBy may already be cleared while orphan map keys remain.
	RemoveNodeFromAllGatewayRelays(ctx, node.Network, node.ID.String())
	if len(node.AutoRelayedPeers) > 0 {
		ResetAutoRelayedPeer(ctx, node)
	}
	if node.IsRelay {
		SetRelayedNodes(false, node.ID.String(), node.RelayedNodes)
	}
	if routingID := InternetExitRoutingNodeID(node); routingID != "" {
		inetNode, err := GetNodeByID(routingID)
		if err == nil {
			clientNodeIDs := []string{}
			for _, inetNodeClientID := range inetNode.InetNodeReq.InetNodeClientIDs {
				if inetNodeClientID == node.ID.String() {
					continue
				}
				clientNodeIDs = append(clientNodeIDs, inetNodeClientID)
			}
			inetNode.InetNodeReq.InetNodeClientIDs = clientNodeIDs
			UpsertNode(&inetNode)
		}
	}
	if node.IsInternetGateway {
		UnsetInternetGw(ctx, node)
	}

	filters := make(map[string]bool)
	if node.Address.IP != nil {
		filters[node.Address.IP.String()] = true
	}
	if node.Address6.IP != nil {
		filters[node.Address6.IP.String()] = true
	}
	nameservers, _ := (&schema.Nameserver{
		NetworkID: node.Network,
	}).ListByNetwork(ctx)
	for _, ns := range nameservers {
		ns.Servers = FilterOutIPs(ns.Servers, filters)
		delete(ns.Nodes, node.ID.String())
		if len(ns.Servers) > 0 {
			_ = ns.Update(ctx)
		} else {
			_ = ns.Delete(ctx)
		}
	}

	detachedCtx := scope.WithContext(db.WithContext(context.Background()), scope.Level(ctx), scope.ID(ctx))
	go RemoveNodeFromAclPolicy(detachedCtx, *node)
	go RemoveNodeFromEgress(*node)
	go RemoveNodeFromEnrollmentKeys(node)
}

func DeleteNode(ctx context.Context, node *models.Node, purge bool) error {
	alreadyDeleted := node.PendingDelete || node.Action == schema.NODE_DELETE
	node.Action = schema.NODE_DELETE

	if !purge && !alreadyDeleted {
		nodeID := node.ID
		node := &schema.Node{
			ID:            nodeID.String(),
			Action:        schema.NODE_DELETE,
			PendingDelete: true,
		}
		err := node.MarkForDeletion(ctx)
		if err != nil {
			return err
		}
		zombieChan(scope.ID(ctx)) <- nodeID
		return nil
	}
	if alreadyDeleted {
		logger.Log(1, "forcibly deleting node", node.ID.String())
	}

	// Before removing an exit routing node: fail-open its clients and detach from
	// internet egress maps so sticky selections are not left pointing at a dead relay.
	FailOpenAndDetachExitRoutingNode(ctx, node)

	cleanupNodeReferences(ctx, node)
	host := &schema.Host{
		ID: node.HostID,
	}
	if err := host.Get(ctx); err != nil {
		logger.Log(1, "no host found for node", node.ID.String(), "deleting..")
		if delErr := DeleteNodeByID(ctx, node); delErr != nil {
			logger.Log(0, "failed to delete node", node.ID.String(), delErr.Error())
			return delErr
		}
		logger.Log(1, "deleted orphaned node (no host record found)", node.ID.String())
		return nil
	}
	if err := DisassociateNodeFromHost(ctx, node, host); err != nil {
		return err
	}
	return nil
}

// GetNodeByHostRef - gets the node by host id and network
func GetNodeByHostRef(ctx context.Context, hostid, network string) (node models.Node, err error) {
	nodes, err := GetNetworkNodes(ctx, network)
	if err != nil {
		return models.Node{}, err
	}
	for _, node := range nodes {
		if node.HostID.String() == hostid && node.Network == network {
			return node, nil
		}
	}
	return models.Node{}, errors.New("node not found")
}

// DeleteNodeByID - deletes a node from database
func DeleteNodeByID(ctx context.Context, node *models.Node) error {
	_node := &schema.Node{
		ID: node.ID.String(),
	}
	err := _node.Delete(ctx)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	err = _node.DeleteViolations(ctx)
	if err != nil {
		return err
	}

	if err = DeleteMetrics(ctx, node.ID.String()); err != nil {
		logger.Log(1, "unable to remove metrics from DB for node", node.ID.String(), err.Error())
	}
	go DeleteNodeMetricsFromPeers(ctx, node.ID.String())
	return nil
}

// GetAllNodes - returns all nodes in the DB
func GetAllNodes(ctx context.Context) ([]models.Node, error) {
	var nodes []models.Node
	_nodes, err := (&schema.Node{}).ListAll(ctx, dbtypes.WithAllPreloads())
	if err != nil {
		return nil, err
	}

	for i := range _nodes {
		node := ConvertSchemaNodeToModelsNodeWithContext(ctx, &_nodes[i], SkipViolations())
		ensureNodeMutex(node)
		nodes = append(nodes, *node)
	}
	attachPostureViolations(ctx, _nodes, nodes)

	return nodes, nil
}

// attachPostureViolations loads violations for all nodes in one query and
// assigns each node's current-cycle violations onto the models.Node slice.
// schemaNodes and modelsNodes must be the same length and aligned by index.
func attachPostureViolations(ctx context.Context, schemaNodes []schema.Node, modelsNodes []models.Node) {
	if len(schemaNodes) == 0 || len(schemaNodes) != len(modelsNodes) {
		return
	}
	ids := make([]string, 0, len(schemaNodes))
	for i := range schemaNodes {
		if schemaNodes[i].ID != "" && schemaNodes[i].PostureCheckLastEvaluationCycleID != "" {
			ids = append(ids, schemaNodes[i].ID)
		}
	}
	if len(ids) == 0 {
		return
	}
	all, err := schema.ListViolationsByNodeIDs(ctx, ids)
	if err != nil {
		slog.Warn("failed to batch-load posture violations", "error", err, "nodes", len(ids))
		return
	}
	// nodeID -> cycleID -> violations
	byNodeCycle := make(map[string]map[string][]models.Violation, len(ids))
	for _, v := range all {
		cycles := byNodeCycle[v.NodeID]
		if cycles == nil {
			cycles = make(map[string][]models.Violation)
			byNodeCycle[v.NodeID] = cycles
		}
		cycles[v.EvaluationCycleID] = append(cycles[v.EvaluationCycleID], models.Violation{
			CheckID:   v.CheckID,
			Name:      v.Name,
			Attribute: v.Attribute,
			Message:   v.Message,
			Severity:  v.Severity,
		})
	}
	for i := range modelsNodes {
		cycleID := schemaNodes[i].PostureCheckLastEvaluationCycleID
		if cycleID == "" {
			continue
		}
		if cycles, ok := byNodeCycle[schemaNodes[i].ID]; ok {
			modelsNodes[i].PostureChecksViolations = cycles[cycleID]
		}
	}
}

func AddStaticNodestoList(ctx context.Context, nodes []models.Node) []models.Node {
	netMap := make(map[string]struct{})
	for _, node := range nodes {
		if _, ok := netMap[node.Network]; ok {
			continue
		}
		if node.IsIngressGateway {
			nodes = append(nodes, GetStaticNodesByNetwork(ctx, schema.NetworkID(node.Network), false)...)
			netMap[node.Network] = struct{}{}
		}
	}
	return nodes
}

func AddStatusToNodes(ctx context.Context, nodes []models.Node, statusCall bool) (nodesWithStatus []models.Node) {
	aclDefaultPolicyStatusMap := make(map[string]bool)
	for _, node := range nodes {
		if _, ok := aclDefaultPolicyStatusMap[node.Network]; !ok {
			// check default policy if all allowed return true
			defaultPolicy, _ := GetDefaultPolicy(ctx, schema.NetworkID(node.Network), models.DevicePolicy)
			aclDefaultPolicyStatusMap[node.Network] = defaultPolicy.Enabled
		}
		if statusCall {
			GetNodeStatus(ctx, &node, aclDefaultPolicyStatusMap[node.Network])
		} else {
			getNodeCheckInStatus(&node, true)
		}

		nodesWithStatus = append(nodesWithStatus, node)
	}
	return
}

// ensureNodeMutex sets node.Mutex if nil. Mutex is never persisted (json:"-"); unmarshaled
// and freshly constructed nodes need a mutex for in-memory coordination of shared maps.
func ensureNodeMutex(node *models.Node) {
	if node == nil {
		return
	}
	if node.Mutex == nil {
		node.Mutex = &sync.Mutex{}
	}
}

// GetRecordKey - get record key
// depricated
func GetRecordKey(id string, network string) (string, error) {
	if id == "" || network == "" {
		return "", errors.New("unable to get record key")
	}
	return id + "###" + network, nil
}

func GetNodeByID(nodeID string) (models.Node, error) {
	_node := &schema.Node{
		ID: nodeID,
	}
	err := _node.Get(db.WithContext(context.TODO()), dbtypes.WithAllPreloads())
	if err != nil {
		return models.Node{}, err
	}

	node := ConvertSchemaNodeToModelsNode(_node)
	ensureNodeMutex(node)
	return *node, nil
}

// GetNodesByIDs fetches all nodes whose IDs are in the given slice in a single
// preloaded query and returns them as a map keyed by node ID. IDs that don't
// resolve to a node row are simply absent from the result map.
//
// This avoids the N+1 (and N^2) query patterns that arise when callers loop
// over peer IDs and call GetNodeByID per peer (e.g. status / connectivity
// checks driven by metrics).
func GetNodesByIDs(ids []string) (map[string]models.Node, error) {
	if len(ids) == 0 {
		return map[string]models.Node{}, nil
	}
	_nodes, err := (&schema.Node{}).ListByIDs(
		db.WithContext(context.TODO()),
		ids,
		dbtypes.WithAllPreloads(),
	)
	if err != nil {
		return nil, err
	}
	ctx := db.WithContext(context.TODO())
	if len(_nodes) > 0 {
		ctx = scope.WithContext(ctx, scope.TenantScope, _nodes[0].TenantID)
	}
	modelsNodes := make([]models.Node, len(_nodes))
	for i := range _nodes {
		n := ConvertSchemaNodeToModelsNodeWithContext(ctx, &_nodes[i], SkipViolations())
		ensureNodeMutex(n)
		modelsNodes[i] = *n
	}
	attachPostureViolations(ctx, _nodes, modelsNodes)

	result := make(map[string]models.Node, len(modelsNodes))
	for i := range modelsNodes {
		result[_nodes[i].ID] = modelsNodes[i]
	}
	return result, nil
}

// GetAllNodesAPI - get all nodes for api usage.
// Location/CountryCode are already filled during schema→models conversion from
// the preloaded Host; do not re-fetch hosts (N+1 on SQLite).
func GetAllNodesAPI(nodes []models.Node) []models.ApiNode {
	apiNodes := make([]models.ApiNode, 0, len(nodes))
	for i := range nodes {
		apiNodes = append(apiNodes, *nodes[i].ConvertToAPINode())
	}
	return apiNodes
}

// GetAllNodesAPIWithLocation - get all nodes for api usage with location.
// Uses values already on the models.Node (from host preload / static node).
func GetAllNodesAPIWithLocation(nodes []models.Node) []models.ApiNode {
	apiNodes := make([]models.ApiNode, 0, len(nodes))
	for i := range nodes {
		newApiNode := nodes[i].ConvertToAPINode()
		if nodes[i].IsStatic {
			newApiNode.Location = nodes[i].StaticNode.Location
		}
		apiNodes = append(apiNodes, *newApiNode)
	}
	return apiNodes
}

// GetNodesStatusAPI - gets nodes status
func GetNodesStatusAPI(nodes []models.Node) map[string]models.ApiNodeStatus {
	apiStatusNodesMap := make(map[string]models.ApiNodeStatus)
	for i := range nodes {
		newApiNode := nodes[i].ConvertToStatusNode()
		apiStatusNodesMap[newApiNode.ID] = *newApiNode
	}
	return apiStatusNodesMap
}

// DeleteExpiredNodes - goroutine which deletes nodes which are expired
func DeleteExpiredNodes(ctx context.Context) {
	// Delete Expired Nodes Every Hour
	ticker := time.NewTicker(time.Hour)
	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			nodes, err := (&schema.Node{}).ListAll(db.WithContext(ctx))
			if err != nil {
				slog.Error("failed to retrieve all nodes", "error", err.Error())
				return
			}
			for _, node := range nodes {
				node := ConvertSchemaNodeToModelsNode(&node)
				if !node.ExpirationDateTime.IsZero() && time.Now().After(node.ExpirationDateTime) {
					DeleteNodesCh <- node
					slog.Info("deleting expired node", "nodeid", node.ID)
				}
			}
		}
	}
}

// SortApiNodes - Sorts slice of ApiNodes by their ID alphabetically with numbers first
func SortApiNodes(unsortedNodes []models.ApiNode) {
	sort.Slice(unsortedNodes, func(i, j int) bool {
		return unsortedNodes[i].ID < unsortedNodes[j].ID
	})
}

func ValidateParams(nodeid, netid string) (models.Node, error) {
	node, err := GetNodeByID(nodeid)
	if err != nil {
		slog.Error("error fetching node", "node", nodeid, "error", err.Error())
		return node, fmt.Errorf("error fetching node during parameter validation: %v", err)
	}
	if node.Network != netid {
		slog.Error("network url param does not match node id", "url nodeid", netid, "node", node.Network)
		return node, fmt.Errorf("network url param does not match node network")
	}
	return node, nil
}

const (
	egressLoopbackIPv4 = "127.0.0.0/8"
	egressLoopbackIPv6 = "::1/128"
)

// ValidateEgressCIDR rejects egress ranges that overlap the Netmaker network
// or loopback space. Empty range and "*" are allowed (domain-only / inet gw).
func ValidateEgressCIDR(network *schema.Network, cidr string) error {
	if cidr == "" || cidr == "*" {
		return nil
	}
	normalized, err := NormalizeCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid egress range: %w", err)
	}
	normNetv4 := network.AddressRange
	if normNetv4 != "" {
		if n, err := NormalizeCIDR(normNetv4); err == nil {
			normNetv4 = n
		}
	}
	normNetv6 := network.AddressRange6
	if normNetv6 != "" {
		if n, err := NormalizeCIDR(normNetv6); err == nil {
			normNetv6 = n
		}
	}
	if normNetv4 != "" && ContainsCIDR(normNetv4, normalized) {
		return errors.New("egress range must not overlap the Netmaker network IPv4 range")
	}
	if normNetv6 != "" && ContainsCIDR(normNetv6, normalized) {
		return errors.New("egress range must not overlap the Netmaker network IPv6 range")
	}
	if ContainsCIDR(egressLoopbackIPv4, normalized) {
		return errors.New("egress range must not include loopback (127.0.0.0/8)")
	}
	if ContainsCIDR(egressLoopbackIPv6, normalized) {
		return errors.New("egress range must not include IPv6 loopback (::1/128)")
	}
	return nil
}

func ValidateEgressRange(ctx context.Context, netID string, ranges []string) error {
	network := &schema.Network{Name: netID}
	err := network.Get(ctx)
	if err != nil {
		slog.Error("error getting network with netid", "error", netID, err.Error)
		return errors.New("error getting network with netid:  " + netID + " " + err.Error())
	}
	for _, v := range ranges {
		if err := ValidateEgressCIDR(network, v); err != nil {
			slog.Error("invalid egress range", "range", v, "error", err.Error())
			return err
		}
	}
	return nil
}

func ContainsCIDR(net1, net2 string) bool {
	_, ipNet1, err1 := net.ParseCIDR(net1)
	_, ipNet2, err2 := net.ParseCIDR(net2)
	if err1 != nil || err2 != nil {
		return false
	}
	return ipNet1.Contains(ipNet2.IP) || ipNet2.Contains(ipNet1.IP)
}

func ConvertSchemaNodeToApiNode(_node *schema.Node) *models.ApiNode {
	return ConvertSchemaNodeToModelsNode(_node).ConvertToAPINode()
}

type nodeConvertOpts struct {
	skipViolations bool
}

// NodeConvertOption customizes schema→models conversion.
type NodeConvertOption func(*nodeConvertOpts)

// SkipViolations skips the per-node posture_check_violations query.
// Use with attachPostureViolations for batched list loads.
func SkipViolations() NodeConvertOption {
	return func(o *nodeConvertOpts) { o.skipViolations = true }
}

func ConvertSchemaNodeToModelsNode(_node *schema.Node, opts ...NodeConvertOption) *models.Node {
	ctx := scope.WithContext(db.WithContext(context.TODO()), scope.TenantScope, _node.TenantID)
	return ConvertSchemaNodeToModelsNodeWithContext(ctx, _node, opts...)
}

func ConvertSchemaNodeToModelsNodeWithContext(ctx context.Context, _node *schema.Node, opts ...NodeConvertOption) *models.Node {
	cfg := nodeConvertOpts{}
	for _, opt := range opts {
		opt(&cfg)
	}

	nodeID, err := uuid.Parse(_node.ID)
	if err != nil {
		return &models.Node{}
	}

	var nodeAddr, nodeAddr6 net.IPNet
	if _node.Address != "" {
		ip, cidr, err := net.ParseCIDR(_node.Address)
		if err != nil {
			return &models.Node{}
		}

		cidr.IP = ip
		nodeAddr = *cidr
	}

	if _node.Address6 != "" {
		ip6, cidr, err := net.ParseCIDR(_node.Address6)
		if err != nil {
			return &models.Node{}
		}

		cidr.IP = ip6
		nodeAddr6 = *cidr
	}

	hostID, err := uuid.Parse(_node.HostID)
	if err != nil {
		return &models.Node{}
	}

	if _node.Host == nil {
		_node.Host = &schema.Host{
			ID: hostID,
		}
		err = _node.Host.Get(ctx)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				_node.Host = &schema.Host{}
			} else {
				return &models.Node{}
			}
		}
	}

	var netAddrRange, netAddr6Range net.IPNet
	if _node.Network == nil {
		_node.Network = &schema.Network{
			ID: _node.NetworkID,
		}
		err = _node.Network.Get(ctx)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				_node.Network = &schema.Network{}
			} else {
				return &models.Node{}
			}
		}
	}

	if _node.Network.AddressRange != "" {
		_, cidr, err := net.ParseCIDR(_node.Network.AddressRange)
		if err != nil {
			return &models.Node{}
		}

		netAddrRange = *cidr
	}

	if _node.Network.AddressRange6 != "" {
		_, cidr, err := net.ParseCIDR(_node.Network.AddressRange6)
		if err != nil {
			return &models.Node{}
		}

		netAddr6Range = *cidr
	}

	var violations []models.Violation
	if !cfg.skipViolations {
		_violations, err := _node.ListViolations(ctx)
		if err == nil {
			for _, _violation := range _violations {
				violations = append(violations, models.Violation{
					CheckID:   _violation.CheckID,
					Name:      _violation.Name,
					Attribute: _violation.Attribute,
					Message:   _violation.Message,
					Severity:  _violation.Severity,
				})
			}
		}
	}

	node := &models.Node{
		CommonNode: models.CommonNode{
			ID:                nodeID,
			TenantID:          _node.TenantID,
			HostID:            hostID,
			Network:           _node.Network.Name,
			NetworkRange:      netAddrRange,
			NetworkRange6:     netAddr6Range,
			Server:            servercfg.GetServer(),
			Connected:         _node.Connected,
			Address:           nodeAddr,
			Address6:          nodeAddr6,
			Action:            _node.Action,
			IsIngressGateway:  _node.IsGateway,
			IsRelay:           _node.IsGateway,
			IsGw:              _node.IsGateway,
			AutoAssignGateway: _node.AutoAssignGateway,
			// TCP proxy listen settings live on the host; surface on the node API for UI/clients.
			TcpProxyEnabled: _node.Host != nil && _node.Host.TcpProxyEnabled,
			TcpProxyListenPort: func() int {
				if _node.Host == nil {
					return 0
				}
				if _node.Host.TcpProxyListenPort > 0 {
					return _node.Host.TcpProxyListenPort
				}
				if _node.Host.TcpProxyEnabled {
					return schema.DefaultTcpProxyListenPort
				}
				return 0
			}(),
			TcpProxyTLSMode: func() string {
				if _node.Host == nil || !_node.Host.TcpProxyEnabled {
					return ""
				}
				if _node.Host.TcpProxyTLSMode != "" {
					return _node.Host.TcpProxyTLSMode
				}
				return schema.TcpProxyTLSModeSelfSigned
			}(),
			TcpProxyListenAddr: func() string {
				if _node.Host != nil {
					return _node.Host.TcpProxyListenAddr
				}
				return ""
			}(),
			TcpProxyPublicHostname: func() string {
				if _node.Host != nil {
					return _node.Host.TcpProxyPublicHostname
				}
				return ""
			}(),
			UseTcpUplink: _node.UseTcpUplink,
		},
		PendingDelete:                      _node.PendingDelete,
		LastModified:                       _node.UpdatedAt,
		LastCheckIn:                        _node.LastCheckIn,
		ExpirationDateTime:                 _node.ExpirationDateTime,
		Metadata:                           _node.Metadata,
		IsAutoRelay:                        _node.IsAutoRelay == "yes",
		AutoRelayedPeers:                   _node.AutoRelayedPeers.Data(),
		IsInternetGateway:                  _node.IsInternetGateway,
		SelectedInternetEgressID:           _node.SelectedInternetEgressID,
		Tags:                               make(map[models.TagID]struct{}),
		Status:                             _node.Status,
		PostureChecksViolations:            violations,
		PostureCheckViolationSeverityLevel: _node.PostureCheckSeverity,
		LastEvaluationCycleID:              _node.PostureCheckLastEvaluationCycleID,
		LastEvaluatedAt:                    _node.PostureCheckLastEvaluatedAt,
		Location:                           _node.Host.Location,
		CountryCode:                        _node.Host.CountryCode,
	}

	if _node.IsGateway {
		node.IngressGatewayRange = _node.Network.AddressRange
		node.IngressGatewayRange6 = _node.Network.AddressRange6
		node.IngressPersistentKeepalive = int32(_node.Host.PersistentKeepalive.Seconds())
		node.IngressMTU = int32(_node.Host.MTU)
		node.AdditionalRagIps = make([]net.IP, 0, len(_node.AdditionalGatewayEndpoints))
		node.RelayedNodes = make([]string, 0, len(_node.RelayedClients))
		node.InetNodeReq = models.InetNodeReq{
			InetNodeClientIDs: make([]string, 0, len(_node.RelayedIGWClients)),
		}

		for relayedClientID := range _node.RelayedClients {
			node.RelayedNodes = append(node.RelayedNodes, relayedClientID)
		}

		for relayedIGWClientID := range _node.RelayedIGWClients {
			node.InetNodeReq.InetNodeClientIDs = append(node.InetNodeReq.InetNodeClientIDs, relayedIGWClientID)
			// Keep RelayedNodes complete for AllowedIPs: exit clients must be
			// advertised even if RelayedClients and RelayedIGWClients diverge.
			if !StringSliceContains(node.RelayedNodes, relayedIGWClientID) {
				node.RelayedNodes = append(node.RelayedNodes, relayedIGWClientID)
			}
		}

		for _, additionalEndpoint := range _node.AdditionalGatewayEndpoints {
			endpointIP := net.ParseIP(additionalEndpoint)
			if endpointIP != nil {
				node.AdditionalRagIps = append(node.AdditionalRagIps, endpointIP)
			}
		}
	} else if len(_node.RelayedClients) > 0 {
		// A non-gateway node can still relay clients when it acts as an internet
		// exit node: the peers using it as their exit node are relayed through it
		// for overlay stability. Surface those relationships (previously only
		// populated for gateways) so peer AllowedIPs and ACL/firewall rules
		// account for them, and so a model->schema round-trip doesn't wipe them.
		node.RelayedNodes = make([]string, 0, len(_node.RelayedClients))
		for relayedClientID := range _node.RelayedClients {
			node.RelayedNodes = append(node.RelayedNodes, relayedClientID)
		}
		node.InetNodeReq = models.InetNodeReq{
			InetNodeClientIDs: make([]string, 0, len(_node.RelayedIGWClients)),
		}
		for relayedIGWClientID := range _node.RelayedIGWClients {
			node.InetNodeReq.InetNodeClientIDs = append(node.InetNodeReq.InetNodeClientIDs, relayedIGWClientID)
			if !StringSliceContains(node.RelayedNodes, relayedIGWClientID) {
				node.RelayedNodes = append(node.RelayedNodes, relayedIGWClientID)
			}
		}
	}

	if _node.RelayedByNodeID != nil && *_node.RelayedByNodeID != "" {
		node.IsRelayed = true
		node.RelayedBy = *_node.RelayedByNodeID

		if _node.IsIGWClient {
			node.InternetGwID = *_node.RelayedByNodeID
		}
	}

	for tagID := range _node.Tags {
		node.Tags[models.TagID(tagID)] = struct{}{}
	}

	return node
}

func ConvertModelsNodeToSchemaNode(node *models.Node) *schema.Node {
	ctx := scope.WithContext(db.WithContext(context.TODO()), scope.TenantScope, node.TenantID)

	var address, address6 string
	if node.Address.IP != nil {
		address = node.Address.String()
	}

	if node.Address6.IP != nil {
		address6 = node.Address6.String()
	}

	host := &schema.Host{
		ID: node.HostID,
	}
	err := host.Get(ctx)
	if err != nil {
		return &schema.Node{}
	}

	network := &schema.Network{
		Name: node.Network,
	}
	err = network.Get(ctx)
	if err != nil {
		return &schema.Node{}
	}

	isAutoRelay := "no"
	if node.IsAutoRelay {
		isAutoRelay = "yes"
	}

	additionalEndpoints := make([]string, 0, len(node.AdditionalRagIps))
	for _, additionalEndpoint := range node.AdditionalRagIps {
		endpointString := additionalEndpoint.String()
		if endpointString != "<nil>" {
			additionalEndpoints = append(additionalEndpoints, endpointString)
		}
	}

	relayedClients := make(datatypes.JSONMap)
	for _, relayedNodeID := range node.RelayedNodes {
		relayedClients[relayedNodeID] = struct{}{}
	}

	relayedIGWClients := make(datatypes.JSONMap)
	for _, inetNodeClientID := range node.InetNodeReq.InetNodeClientIDs {
		relayedIGWClients[inetNodeClientID] = struct{}{}
	}

	var relayingNodeID *string
	if node.IsRelayed && node.RelayedBy != "" {
		relayedBy := node.RelayedBy
		relayingNodeID = &relayedBy
	}

	tags := make(datatypes.JSONMap)
	for tagID := range node.Tags {
		tags[tagID.String()] = struct{}{}
	}

	return &schema.Node{
		ID:                                node.ID.String(),
		TenantID:                          node.TenantID,
		HostID:                            host.ID.String(),
		Host:                              host,
		NetworkID:                         network.ID,
		Network:                           network,
		Address:                           address,
		Address6:                          address6,
		Connected:                         node.Connected,
		Action:                            node.Action,
		Status:                            node.Status,
		PendingDelete:                     node.PendingDelete,
		AutoAssignGateway:                 node.AutoAssignGateway,
		IsGateway:                         node.IsGw,
		IsAutoRelay:                       isAutoRelay,
		IsInternetGateway:                 node.IsGw && node.IsInternetGateway,
		AdditionalGatewayEndpoints:        additionalEndpoints,
		RelayedClients:                    relayedClients,
		RelayedIGWClients:                 relayedIGWClients,
		RelayedByNodeID:                   relayingNodeID,
		IsIGWClient:                       node.IsRelayed && node.InternetGwID != "",
		UseTcpUplink:                      node.UseTcpUplink,
		SelectedInternetEgressID:          node.SelectedInternetEgressID,
		AutoRelayedPeers:                  datatypes.NewJSONType(node.AutoRelayedPeers),
		Tags:                              tags,
		PostureCheckSeverity:              node.PostureCheckViolationSeverityLevel,
		PostureCheckLastEvaluationCycleID: node.LastEvaluationCycleID,
		PostureCheckLastEvaluatedAt:       node.LastEvaluatedAt,
		Metadata:                          node.Metadata,
		LastCheckIn:                       node.LastCheckIn,
		ExpirationDateTime:                node.ExpirationDateTime,
		CreatedAt:                         node.LastModified,
		UpdatedAt:                         node.LastModified,
	}
}
