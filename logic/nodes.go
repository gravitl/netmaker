package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"slices"
	"sort"
	"sync"
	"time"

	validator "github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/validation"
	"github.com/seancfoley/ipaddress-go/ipaddr"
	"golang.org/x/exp/slog"
)

var (
	nodeCacheMutex        = &sync.RWMutex{}
	nodeNetworkCacheMutex = &sync.RWMutex{}
	nodesCacheMap         = make(map[string]models.Node)
	nodesNetworkCacheMap  = make(map[string]map[string]models.Node)
	DeleteNodesCh         = make(chan *models.Node, 100)
)

func getNodeFromCache(nodeID string) (node models.Node, ok bool) {
	nodeCacheMutex.RLock()
	node, ok = nodesCacheMap[nodeID]
	if node.Mutex == nil {
		node.Mutex = &sync.Mutex{}
	}
	nodeCacheMutex.RUnlock()
	return
}
func getNodesFromCache() (nodes []models.Node) {
	nodeCacheMutex.RLock()
	for _, node := range nodesCacheMap {
		if node.Mutex == nil {
			node.Mutex = &sync.Mutex{}
		}
		nodes = append(nodes, node)
	}
	nodeCacheMutex.RUnlock()
	return
}

func deleteNodeFromCache(nodeID string) {
	nodeCacheMutex.Lock()
	delete(nodesCacheMap, nodeID)
	nodeCacheMutex.Unlock()
}
func deleteNodeFromNetworkCache(nodeID string, network string) {
	nodeNetworkCacheMutex.Lock()
	delete(nodesNetworkCacheMap[network], nodeID)
	nodeNetworkCacheMutex.Unlock()
}

func storeNodeInNetworkCache(node models.Node, network string) {
	nodeNetworkCacheMutex.Lock()
	if nodesNetworkCacheMap[network] == nil {
		nodesNetworkCacheMap[network] = make(map[string]models.Node)
	}
	nodesNetworkCacheMap[network][node.ID.String()] = node
	nodeNetworkCacheMutex.Unlock()
}

func storeNodeInCache(node models.Node) {
	nodeCacheMutex.Lock()
	nodesCacheMap[node.ID.String()] = node
	nodeCacheMutex.Unlock()
}
func loadNodesIntoNetworkCache(nMap map[string]models.Node) {
	nodeNetworkCacheMutex.Lock()
	for _, v := range nMap {
		network := v.Network
		if nodesNetworkCacheMap[network] == nil {
			nodesNetworkCacheMap[network] = make(map[string]models.Node)
		}
		nodesNetworkCacheMap[network][v.ID.String()] = v
	}
	nodeNetworkCacheMutex.Unlock()
}

func loadNodesIntoCache(nMap map[string]models.Node) {
	nodeCacheMutex.Lock()
	nodesCacheMap = nMap
	nodeCacheMutex.Unlock()
}
func ClearNodeCache() {
	nodeCacheMutex.Lock()
	nodesCacheMap = make(map[string]models.Node)
	nodesNetworkCacheMap = make(map[string]map[string]models.Node)
	nodeCacheMutex.Unlock()
}

const (
	// RELAY_NODE_ERR - error to return if relay node is unfound
	RELAY_NODE_ERR = "could not find relay for node"
	// NodePurgeTime time to wait for node to response to a NODE_DELETE actions
	NodePurgeTime = time.Second * 10
	// NodePurgeCheckTime is how often to check nodes for Pending Delete
	NodePurgeCheckTime = time.Second * 30
)

// GetNetworkNodes - gets the nodes of a network
func GetNetworkNodes(network string) ([]models.Node, error) {

	if networkNodes, ok := nodesNetworkCacheMap[network]; ok {
		nodeNetworkCacheMutex.Lock()
		defer nodeNetworkCacheMutex.Unlock()
		return slices.Collect(maps.Values(networkNodes)), nil
	}
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

	if networkNodes, ok := nodesNetworkCacheMap[network]; ok {
		nodeNetworkCacheMutex.Lock()
		defer nodeNetworkCacheMutex.Unlock()
		return slices.Collect(maps.Values(networkNodes))
	}
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
	pendingCheckins   = make(map[string]models.Node)
	pendingCheckinsMu sync.Mutex
)

// UpdateNodeCheckin - buffers the checkin timestamp in memory when caching is enabled.
// The actual DB write is deferred to FlushNodeCheckins (every 30s).
// When caching is disabled (HA mode), writes directly to the DB.
func UpdateNodeCheckin(node *models.Node) error {
	node.SetLastCheckIn()
	node.EgressDetails = models.EgressDetails{}
	if servercfg.CacheEnabled() {
		pendingCheckinsMu.Lock()
		pendingCheckins[node.ID.String()] = *node
		pendingCheckinsMu.Unlock()
		storeNodeInCache(*node)
		storeNodeInNetworkCache(*node, node.Network)
		return nil
	}
	data, err := json.Marshal(node)
	if err != nil {
		return err
	}
	return database.Insert(node.ID.String(), string(data), database.NODES_TABLE_NAME)
}

// FlushNodeCheckins - writes all buffered check-in updates to the DB in one batch.
// Called periodically (e.g., every 30s) to avoid per-checkin write lock contention.
func FlushNodeCheckins() {
	pendingCheckinsMu.Lock()
	batch := pendingCheckins
	pendingCheckins = make(map[string]models.Node)
	pendingCheckinsMu.Unlock()
	if len(batch) == 0 {
		return
	}
	var failed int
	for id, node := range batch {
		data, err := json.Marshal(node)
		if err != nil {
			failed++
			continue
		}
		if err := database.Insert(id, string(data), database.NODES_TABLE_NAME); err != nil {
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
	err = database.Insert(newNode.ID.String(), string(data), database.NODES_TABLE_NAME)
	if err != nil {
		return err
	}
	if servercfg.CacheEnabled() {
		storeNodeInCache(*newNode)
		storeNodeInNetworkCache(*newNode, newNode.Network)
	}
	return nil
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

	// check for un-settable server values
	if err := ValidateNode(newNode, true); err != nil {
		return err
	}

	if newNode.ID == currentNode.ID {
		newNode.EgressDetails = models.EgressDetails{}
		newNode.SetLastModified()
		if !currentNode.Connected && newNode.Connected {
			newNode.SetLastCheckIn()
		}
		if data, err := json.Marshal(newNode); err != nil {
			return err
		} else {
			err = database.Insert(newNode.ID.String(), string(data), database.NODES_TABLE_NAME)
			if err != nil {
				return err
			}
			if servercfg.CacheEnabled() {
				storeNodeInCache(*newNode)
				storeNodeInNetworkCache(*newNode, newNode.Network)
				if newNode.Address.IP != nil && !newNode.Address.IP.Equal(currentNode.Address.IP) {
					AddIpToAllocatedIpMap(newNode.Network, newNode.Address.IP)
					RemoveIpFromAllocatedIpMap(currentNode.Network, currentNode.Address.IP.String())
				}
				if newNode.Address6.IP != nil && !newNode.Address6.IP.Equal(currentNode.Address6.IP) {
					AddIpToAllocatedIpMap(newNode.Network, newNode.Address6.IP)
					RemoveIpFromAllocatedIpMap(currentNode.Network, currentNode.Address6.IP.String())
				}
			}
			return nil
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
	if node.FailedOverBy != uuid.Nil {
		ResetFailedOverPeer(node)
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
	alreadyDeleted := node.PendingDelete || node.Action == models.NODE_DELETE
	node.Action = models.NODE_DELETE

	if !purge && !alreadyDeleted {
		newnode := *node
		newnode.PendingDelete = true
		if err := UpdateNode(node, &newnode); err != nil {
			return err
		}
		newZombie <- node.ID
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
	var err error
	var key = node.ID.String()
	if err = database.DeleteRecord(database.NODES_TABLE_NAME, key); err != nil {
		if !database.IsEmptyRecord(err) {
			return err
		}
	}
	if servercfg.CacheEnabled() {
		deleteNodeFromCache(node.ID.String())
		deleteNodeFromNetworkCache(node.ID.String(), node.Network)
	}
	if servercfg.IsDNSMode() {
		SetDNS()
	}
	// removeZombie <- node.ID
	if err = DeleteMetrics(node.ID.String()); err != nil {
		logger.Log(1, "unable to remove metrics from DB for node", node.ID.String(), err.Error())
	}
	//recycle ip address
	if servercfg.CacheEnabled() {
		if node.Address.IP != nil {
			RemoveIpFromAllocatedIpMap(node.Network, node.Address.IP.String())
		}
		if node.Address6.IP != nil {
			RemoveIpFromAllocatedIpMap(node.Network, node.Address6.IP.String())
		}
	}

	return nil
}

// IsNodeIDUnique - checks if node id is unique
func IsNodeIDUnique(node *models.Node) (bool, error) {
	_, err := database.FetchRecord(database.NODES_TABLE_NAME, node.ID.String())
	return database.IsEmptyRecord(err), err
}

// ValidateNode - validates node values
func ValidateNode(node *models.Node, isUpdate bool) error {
	v := validator.New()
	_ = v.RegisterValidation("id_unique", func(fl validator.FieldLevel) bool {
		if isUpdate {
			return true
		}
		isFieldUnique, _ := IsNodeIDUnique(node)
		return isFieldUnique
	})
	_ = v.RegisterValidation("network_exists", func(fl validator.FieldLevel) bool {
		err := (&schema.Network{Name: node.Network}).Get(db.WithContext(context.TODO()))
		return err == nil
	})
	_ = v.RegisterValidation("checkyesornoorunset", func(f1 validator.FieldLevel) bool {
		return validation.CheckYesOrNoOrUnset(f1)
	})
	err := v.Struct(node)
	return err
}

// GetAllNodes - returns all nodes in the DB
func GetAllNodes() ([]models.Node, error) {
	var nodes []models.Node
	if servercfg.CacheEnabled() {
		nodes = getNodesFromCache()
		if len(nodes) != 0 {
			return nodes, nil
		}
	}
	nodesMap := make(map[string]models.Node)
	if servercfg.CacheEnabled() {
		defer loadNodesIntoCache(nodesMap)
		defer loadNodesIntoNetworkCache(nodesMap)
	}
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
		// add node to our array
		nodes = append(nodes, node)
		if node.Mutex == nil {
			node.Mutex = &sync.Mutex{}
		}
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

// SetNodeDefaults - sets the defaults of a node to avoid empty fields
func SetNodeDefaults(node *models.Node, resetConnected bool) {
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
	if servercfg.CacheEnabled() {
		if node, ok := getNodeFromCache(uuid); ok {
			return node, nil
		}
	}
	var record, err = database.FetchRecord(database.NODES_TABLE_NAME, uuid)
	if err != nil {
		return models.Node{}, err
	}
	var node models.Node
	if err = json.Unmarshal([]byte(record), &node); err != nil {
		return models.Node{}, err
	}
	if servercfg.CacheEnabled() {
		storeNodeInCache(node)
		storeNodeInNetworkCache(node, node.Network)
	}
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

// FindRelay - returns the node that is the relay for a relayed node
func FindRelay(node *models.Node) *models.Node {
	relay, err := GetNodeByID(node.RelayedBy)
	if err != nil {
		logger.Log(0, "FindRelay: "+err.Error())
		return nil
	}
	return &relay
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
			allnodes, err := GetAllNodes()
			if err != nil {
				slog.Error("failed to retrieve all nodes", "error", err.Error())
				return
			}
			for _, node := range allnodes {
				node := node
				if time.Now().After(node.ExpirationDateTime) {
					DeleteNodesCh <- &node
					slog.Info("deleting expired node", "nodeid", node.ID.String())
				}
			}
		}
	}
}

// createNode - creates a node in database
func createNode(node *models.Node) error {
	// lock because we need unique IPs and having it concurrent makes parallel calls result in same "unique" IPs
	addressLock.Lock()
	defer addressLock.Unlock()

	host := &schema.Host{
		ID: node.HostID,
	}
	err := host.Get(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}

	SetNodeDefaults(node, true)
	parentNetwork := &schema.Network{Name: node.Network}
	err = parentNetwork.Get(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}
	if node.Address.IP == nil {
		if parentNetwork.AddressRange != "" {
			if node.Address.IP, err = UniqueAddress(node.Network, false); err != nil {
				return err
			}
			_, cidr, err := net.ParseCIDR(parentNetwork.AddressRange)
			if err != nil {
				return err
			}
			node.Address.Mask = net.CIDRMask(cidr.Mask.Size())
		}
	} else if !IsIPUnique(node.Network, node.Address.String(), database.NODES_TABLE_NAME, false) {
		return fmt.Errorf("invalid address: ipv4 %s is not unique", node.Address.String())
	}
	if node.Address6.IP == nil {
		if parentNetwork.AddressRange6 != "" {
			if node.Address6.IP, err = UniqueAddress6(node.Network, false); err != nil {
				return err
			}
			_, cidr, err := net.ParseCIDR(parentNetwork.AddressRange6)
			if err != nil {
				return err
			}
			node.Address6.Mask = net.CIDRMask(cidr.Mask.Size())
		}
	} else if !IsIPUnique(node.Network, node.Address6.String(), database.NODES_TABLE_NAME, true) {
		return fmt.Errorf("invalid address: ipv6 %s is not unique", node.Address6.String())
	}
	node.ID = uuid.New()
	//Create a JWT for the node
	tokenString, _ := CreateJWT(node.ID.String(), host.MacAddress.String(), node.Network)
	if tokenString == "" {
		//logic.ReturnErrorResponse(w, r, errorResponse)
		return err
	}
	err = ValidateNode(node, false)
	if err != nil {
		return err
	}
	CheckZombies(node)
	node.SetLastCheckIn()
	nodebytes, err := json.Marshal(&node)
	if err != nil {
		return err
	}
	err = database.Insert(node.ID.String(), string(nodebytes), database.NODES_TABLE_NAME)
	if err != nil {
		return err
	}
	if servercfg.CacheEnabled() {
		storeNodeInCache(*node)
		storeNodeInNetworkCache(*node, node.Network)
		if node.Address.IP != nil {
			AddIpToAllocatedIpMap(node.Network, node.Address.IP)
		}
		if node.Address6.IP != nil {
			AddIpToAllocatedIpMap(node.Network, node.Address6.IP)
		}
	}

	if err = UpdateMetrics(node.ID.String(), &models.Metrics{Connectivity: make(map[string]models.Metric)}); err != nil {
		logger.Log(1, "failed to initialize metrics for node", node.ID.String(), err.Error())
	}

	SetNetworkNodesLastModified(node.Network)
	if servercfg.IsDNSMode() {
		err = SetDNS()
	}
	return err
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

func ValidateNodeIp(currentNode *models.Node, newNode *models.ApiNode) error {

	if currentNode.Address.IP != nil && currentNode.Address.String() != newNode.Address {
		if !IsIPUnique(newNode.Network, newNode.Address, database.NODES_TABLE_NAME, false) ||
			!IsIPUnique(newNode.Network, newNode.Address, database.EXT_CLIENT_TABLE_NAME, false) {
			return errors.New("ip specified is already allocated:  " + newNode.Address)
		}
	}
	if currentNode.Address6.IP != nil && currentNode.Address6.String() != newNode.Address6 {
		if !IsIPUnique(newNode.Network, newNode.Address6, database.NODES_TABLE_NAME, false) ||
			!IsIPUnique(newNode.Network, newNode.Address6, database.EXT_CLIENT_TABLE_NAME, false) {
			return errors.New("ip specified is already allocated:  " + newNode.Address6)
		}
	}

	return nil
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

// GetAllFailOvers - gets all the nodes that are failovers
func GetAllFailOvers() ([]models.Node, error) {
	nodes, err := GetAllNodes()
	if err != nil {
		return nil, err
	}
	igs := make([]models.Node, 0)
	for _, node := range nodes {
		if node.IsFailOver {
			igs = append(igs, node)
		}
	}
	return igs, nil
}
