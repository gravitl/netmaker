package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/seancfoley/ipaddress-go/ipaddr"
	"golang.org/x/exp/slog"
	"gorm.io/gorm"
)

var (
	DeleteNodesCh = make(chan *schema.Node, 100)
)

// GetNetworkNodes - gets the nodes of a network
func GetNetworkNodes(network string) ([]models.Node, error) {
	allnodes, err := GetAllNodes()
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
func FlushNodeCheckins() {
	pendingCheckinsMu.Lock()
	batch := pendingCheckins
	pendingCheckins = make(map[string]time.Time)
	pendingCheckinsMu.Unlock()
	if len(batch) == 0 {
		return
	}
	var failed int
	for id, checkin := range batch {
		node := &schema.Node{
			ID:          id,
			LastCheckIn: checkin,
		}
		err := node.UpdateLastCheckIn(db.WithContext(context.TODO()))
		if err != nil {
			failed++
		}
	}
	if failed > 0 {
		slog.Error("FlushNodeCheckins: failed to persist checkins", "failed", failed, "total", len(batch))
	}
}

// UpsertNode - updates node in the DB
func UpsertNode(newNode *models.Node) error {
	newNode.SetLastModified()
	data, err := json.Marshal(newNode)
	if err != nil {
		return err
	}
	newNode.EgressDetails = models.EgressDetails{}
	return database.Insert(newNode.ID.String(), string(data), database.NODES_TABLE_NAME)
}

// UpdateNode - takes a node and updates another node with it's values
func UpdateNode(currentNode *models.Node, newNode *models.Node) error {
	if newNode.Address.IP.String() != currentNode.Address.IP.String() {
		network := &schema.Network{Name: newNode.Network}
		if err := network.Get(db.WithContext(context.TODO())); err == nil {
			if !IsAddressInCIDR(newNode.Address.IP, network.AddressRange) {
				return fmt.Errorf("invalid address provided; out of network range for node %s", newNode.ID)
			}
		}
	}
	newNode.Fill(currentNode, servercfg.IsPro)

	if newNode.ID == currentNode.ID {
		newNode.EgressDetails = models.EgressDetails{}
		newNode.SetLastModified()
		if !currentNode.Connected && newNode.Connected {
			newNode.SetLastCheckIn()
		}
		if data, err := json.Marshal(newNode); err != nil {
			return err
		} else {
			return database.Insert(newNode.ID.String(), string(data), database.NODES_TABLE_NAME)
		}
	}

	return fmt.Errorf("failed to update node %s, cannot change ID", currentNode.ID.String())
}

// DeleteNode - marks node for deletion (and adds to zombie list) if called by UI or deletes node if called by node
// cleanupNodeReferences handles best-effort cleanup of all external references
// to a node (relay, internet gw, failover, nameservers, ACL, egress, enrollment keys).
// Errors are logged but do not prevent node deletion.
func cleanupNodeReferences(node *models.Node) {
	if node.IsIngressGateway {
		if err := DeleteGatewayExtClients(node.ID.String(), node.Network); err != nil {
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
	if len(node.AutoRelayedPeers) > 0 {
		ResetAutoRelayedPeer(node)
	}
	if node.IsRelay {
		SetRelayedNodes(false, node.ID.String(), node.RelayedNodes)
	}
	if node.InternetGwID != "" {
		inetNode, err := GetNodeByID(node.InternetGwID)
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
		UnsetInternetGw(node)
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
	}).ListByNetwork(db.WithContext(context.TODO()))
	for _, ns := range nameservers {
		ns.Servers = FilterOutIPs(ns.Servers, filters)
		if len(ns.Servers) > 0 {
			_ = ns.Update(db.WithContext(context.TODO()))
		} else {
			_ = ns.Delete(db.WithContext(context.TODO()))
		}
	}

	go RemoveNodeFromAclPolicy(*node)
	go RemoveNodeFromEgress(*node)
	go RemoveNodeFromEnrollmentKeys(node)
}

func DeleteNode(node *models.Node, purge bool) error {
	alreadyDeleted := node.PendingDelete || node.Action == schema.NODE_DELETE
	node.Action = schema.NODE_DELETE

	if !purge && !alreadyDeleted {
		nodeID := node.ID
		node := &schema.Node{
			ID:            nodeID.String(),
			Action:        schema.NODE_DELETE,
			PendingDelete: true,
		}
		err := node.MarkForDeletion(db.WithContext(context.TODO()))
		if err != nil {
			return err
		}
		newZombie <- nodeID
		return nil
	}
	if alreadyDeleted {
		logger.Log(1, "forcibly deleting node", node.ID.String())
	}
	cleanupNodeReferences(node)
	host := &schema.Host{
		ID: node.HostID,
	}
	if err := host.Get(db.WithContext(context.TODO())); err != nil {
		logger.Log(1, "no host found for node", node.ID.String(), "deleting..")
		if delErr := DeleteNodeByID(node); delErr != nil {
			logger.Log(0, "failed to delete node", node.ID.String(), delErr.Error())
			return delErr
		}
		logger.Log(1, "deleted orphaned node (no host record found)", node.ID.String())
		return nil
	}
	if err := DissasociateNodeFromHost(node, host); err != nil {
		return err
	}
	return nil
}

// GetNodeByHostRef - gets the node by host id and network
func GetNodeByHostRef(hostid, network string) (node models.Node, err error) {
	nodes, err := GetNetworkNodes(network)
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
func DeleteNodeByID(node *models.Node) error {
	_node := &schema.Node{
		ID: node.ID.String(),
	}
	err := _node.Delete(db.WithContext(context.TODO()))
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if err = DeleteMetrics(node.ID.String()); err != nil {
		logger.Log(1, "unable to remove metrics from DB for node", node.ID.String(), err.Error())
	}
	return nil
}

// GetAllNodes - returns all nodes in the DB
func GetAllNodes() ([]models.Node, error) {
	var nodes []models.Node
	nodesMap := make(map[string]models.Node)
	collection, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		if database.IsEmptyRecord(err) {
			return []models.Node{}, nil
		}
		return []models.Node{}, err
	}

	for _, value := range collection {
		var node models.Node
		// ignore legacy nodes in database
		if err := json.Unmarshal([]byte(value), &node); err != nil {
			logger.Log(3, "legacy node detected: ", err.Error())
			continue
		}
		ensureNodeMutex(&node)
		// add node to our array
		nodes = append(nodes, node)
		nodesMap[node.ID.String()] = node
	}

	return nodes, nil
}

func AddStaticNodestoList(nodes []models.Node) []models.Node {
	netMap := make(map[string]struct{})
	for _, node := range nodes {
		if _, ok := netMap[node.Network]; ok {
			continue
		}
		if node.IsIngressGateway {
			nodes = append(nodes, GetStaticNodesByNetwork(schema.NetworkID(node.Network), false)...)
			netMap[node.Network] = struct{}{}
		}
	}
	return nodes
}

func AddStatusToNodes(nodes []models.Node, statusCall bool) (nodesWithStatus []models.Node) {
	aclDefaultPolicyStatusMap := make(map[string]bool)
	for _, node := range nodes {
		if _, ok := aclDefaultPolicyStatusMap[node.Network]; !ok {
			// check default policy if all allowed return true
			defaultPolicy, _ := GetDefaultPolicy(schema.NetworkID(node.Network), models.DevicePolicy)
			aclDefaultPolicyStatusMap[node.Network] = defaultPolicy.Enabled
		}
		if statusCall {
			GetNodeStatus(&node, aclDefaultPolicyStatusMap[node.Network])
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

// SetNodeDefaults - sets the defaults of a node to avoid empty fields
func SetNodeDefaults(node *models.Node, resetConnected bool) {
	ensureNodeMutex(node)
	parentNetwork := &schema.Network{Name: node.Network}
	_ = parentNetwork.Get(db.WithContext(context.TODO()))
	_, cidr, err := net.ParseCIDR(parentNetwork.AddressRange)
	if err == nil {
		node.NetworkRange = *cidr
	}
	_, cidr, err = net.ParseCIDR(parentNetwork.AddressRange6)
	if err == nil {
		node.NetworkRange6 = *cidr
	}

	if node.FailOverPeers == nil {
		node.FailOverPeers = make(map[string]struct{})
	}

	node.SetLastModified()
	//node.SetLastCheckIn()

	if resetConnected {
		node.SetDefaultConnected()
	}
	node.SetExpirationDateTime()
	if node.Tags == nil {
		node.Tags = make(map[models.TagID]struct{})
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

func GetNodeByID(uuid string) (models.Node, error) {
	var record, err = database.FetchRecord(database.NODES_TABLE_NAME, uuid)
	if err != nil {
		return models.Node{}, err
	}
	var node models.Node
	if err = json.Unmarshal([]byte(record), &node); err != nil {
		return models.Node{}, err
	}
	ensureNodeMutex(&node)
	return node, nil
}

// GetDeletedNodeByID - get a deleted node
func GetDeletedNodeByID(uuid string) (models.Node, error) {

	var node models.Node

	record, err := database.FetchRecord(database.DELETED_NODES_TABLE_NAME, uuid)
	if err != nil {
		return models.Node{}, err
	}

	if err = json.Unmarshal([]byte(record), &node); err != nil {
		return models.Node{}, err
	}

	SetNodeDefaults(&node, true)

	return node, nil
}

// GetAllNodesAPI - get all nodes for api usage
func GetAllNodesAPI(nodes []models.Node) []models.ApiNode {
	apiNodes := []models.ApiNode{}
	for i := range nodes {
		node := nodes[i]
		if !node.IsStatic {
			h := &schema.Host{
				ID: node.HostID,
			}
			err := h.Get(db.WithContext(context.TODO()))
			if err == nil {
				node.Location = h.Location
				node.CountryCode = h.CountryCode
			}
		}
		newApiNode := node.ConvertToAPINode()
		apiNodes = append(apiNodes, *newApiNode)
	}
	return apiNodes[:]
}

// GetAllNodesAPI - get all nodes for api usage
func GetAllNodesAPIWithLocation(nodes []models.Node) []models.ApiNode {
	apiNodes := []models.ApiNode{}
	for i := range nodes {
		node := nodes[i]
		newApiNode := node.ConvertToAPINode()
		if node.IsStatic {
			newApiNode.Location = node.StaticNode.Location
		} else {
			host := &schema.Host{
				ID: node.HostID,
			}
			_ = host.Get(db.WithContext(context.TODO()))
			newApiNode.Location = host.Location
		}

		apiNodes = append(apiNodes, *newApiNode)
	}
	return apiNodes[:]
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
				node := node
				if time.Now().After(node.ExpirationDateTime) {
					DeleteNodesCh <- &node
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

func ValidateEgressRange(netID string, ranges []string) error {
	network := &schema.Network{Name: netID}
	err := network.Get(db.WithContext(context.TODO()))
	if err != nil {
		slog.Error("error getting network with netid", "error", netID, err.Error)
		return errors.New("error getting network with netid:  " + netID + " " + err.Error())
	}
	ipv4Net := network.AddressRange
	ipv6Net := network.AddressRange6

	for _, v := range ranges {
		if ipv4Net != "" {
			if ContainsCIDR(ipv4Net, v) {
				slog.Error("egress range should not be the same as or contained in the netmaker network address", "error", v, ipv4Net)
				return errors.New("egress range should not be the same as or contained in the netmaker network address" + v + " " + ipv4Net)
			}
		}
		if ipv6Net != "" {
			if ContainsCIDR(ipv6Net, v) {
				slog.Error("egress range should not be the same as or contained in the netmaker network address", "error", v, ipv6Net)
				return errors.New("egress range should not be the same as or contained in the netmaker network address" + v + " " + ipv6Net)
			}
		}
	}

	return nil
}

func ContainsCIDR(net1, net2 string) bool {
	one, two := ipaddr.NewIPAddressString(net1),
		ipaddr.NewIPAddressString(net2)
	return one.Contains(two) || two.Contains(one)
}

// TODO: implement

func ConvertSchemaNodeToApiNode(_node *schema.Node) *models.ApiNode {
	return &models.ApiNode{
		ID: _node.ID,
	}
}

func ConvertSchemaNodeToModelsNode(_node *schema.Node) *models.Node {
	return &models.Node{
		CommonNode: models.CommonNode{
			ID: uuid.MustParse(_node.ID),
		},
	}
}
