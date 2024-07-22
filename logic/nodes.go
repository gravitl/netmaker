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

	validator "github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/gravitl/netmaker/logic/acls/nodeacls"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/validation"
	"github.com/seancfoley/ipaddress-go/ipaddr"
	"golang.org/x/exp/slog"
)

var (
	nodeCacheMutex = &sync.RWMutex{}
	nodesCacheMap  = make(map[string]models.Node)
)

func getNodeFromCache(nodeID string) (node models.Node, ok bool) {
	nodeCacheMutex.RLock()
	node, ok = nodesCacheMap[nodeID]
	nodeCacheMutex.RUnlock()
	return
}
func getNodesFromCache() (nodes []models.Node) {
	nodeCacheMutex.RLock()
	for _, node := range nodesCacheMap {
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

func storeNodeInCache(node models.Node) {
	nodeCacheMutex.Lock()
	nodesCacheMap[node.ID.String()] = node
	nodeCacheMutex.Unlock()
}

func loadNodesIntoCache(nMap map[string]models.Node) {
	nodeCacheMutex.Lock()
	nodesCacheMap = nMap
	nodeCacheMutex.Unlock()
}
func ClearNodeCache() {
	nodeCacheMutex.Lock()
	nodesCacheMap = make(map[string]models.Node)
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
	allnodes, err := GetAllNodes()
	if err != nil {
		return []models.Node{}, err
	}

	return GetNetworkNodesMemory(allnodes, network), nil
}

// GetHostNodes - fetches all nodes part of the host
func GetHostNodes(host *models.Host) []models.Node {
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
	var nodes = []models.Node{}
	for i := range allNodes {
		node := allNodes[i]
		if node.Network == network {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// UpdateNodeCheckin - updates the checkin time of a node
func UpdateNodeCheckin(node *models.Node) error {
	node.SetLastCheckIn()
	data, err := json.Marshal(node)
	if err != nil {
		return err
	}
	err = database.Insert(node.ID.String(), string(data), database.NODES_TABLE_NAME)
	if err != nil {
		return err
	}
	if servercfg.CacheEnabled() {
		storeNodeInCache(*node)
	}
	return nil
}

// UpsertNode - updates node in the DB
func UpsertNode(newNode *models.Node) error {
	newNode.SetLastModified()
	data, err := json.Marshal(newNode)
	if err != nil {
		return err
	}
	err = database.Insert(newNode.ID.String(), string(data), database.NODES_TABLE_NAME)
	if err != nil {
		return err
	}
	if servercfg.CacheEnabled() {
		storeNodeInCache(*newNode)
	}
	return nil
}

// UpdateNode - takes a node and updates another node with it's values
func UpdateNode(currentNode *models.Node, newNode *models.Node) error {
	if newNode.Address.IP.String() != currentNode.Address.IP.String() {
		if network, err := GetParentNetwork(newNode.Network); err == nil {
			if !IsAddressInCIDR(newNode.Address.IP, network.AddressRange) {
				return fmt.Errorf("invalid address provided; out of network range for node %s", newNode.ID)
			}
		}
	}
	nodeACLDelta := currentNode.DefaultACL != newNode.DefaultACL
	newNode.Fill(currentNode, servercfg.IsPro)

	// check for un-settable server values
	if err := ValidateNode(newNode, true); err != nil {
		return err
	}

	if newNode.ID == currentNode.ID {
		if nodeACLDelta {
			if err := UpdateProNodeACLs(newNode); err != nil {
				logger.Log(1, "failed to apply node level ACLs during creation of node", newNode.ID.String(), "-", err.Error())
				return err
			}
		}

		newNode.SetLastModified()
		if data, err := json.Marshal(newNode); err != nil {
			return err
		} else {
			err = database.Insert(newNode.ID.String(), string(data), database.NODES_TABLE_NAME)
			if err != nil {
				return err
			}
			if servercfg.CacheEnabled() {
				storeNodeInCache(*newNode)
			}
			return nil
		}
	}

	return fmt.Errorf("failed to update node " + currentNode.ID.String() + ", cannot change ID.")
}

// DeleteNode - marks node for deletion (and adds to zombie list) if called by UI or deletes node if called by node
func DeleteNode(node *models.Node, purge bool) error {
	alreadyDeleted := node.PendingDelete || node.Action == models.NODE_DELETE
	node.Action = models.NODE_DELETE
	//delete ext clients if node is ingress gw
	if node.IsIngressGateway {
		if err := DeleteGatewayExtClients(node.ID.String(), node.Network); err != nil {
			slog.Error("failed to delete ext clients", "nodeid", node.ID.String(), "error", err.Error())
		}
	}
	if node.IsRelayed {
		// cleanup node from relayednodes on relay node
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
	if node.IsRelay {
		// unset all the relayed nodes
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
	host, err := GetHost(node.HostID.String())
	if err != nil {
		logger.Log(1, "no host found for node", node.ID.String(), "deleting..")
		if delErr := DeleteNodeByID(node); delErr != nil {
			logger.Log(0, "failed to delete node", node.ID.String(), delErr.Error())
		}
		return err
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
	}
	if servercfg.IsDNSMode() {
		SetDNS()
	}
	_, err = nodeacls.RemoveNodeACL(nodeacls.NetworkID(node.Network), nodeacls.NodeID(node.ID.String()))
	if err != nil {
		// ignoring for now, could hit a nil pointer if delete called twice
		logger.Log(2, "attempted to remove node ACL for node", node.ID.String())
	}
	// removeZombie <- node.ID
	if err = DeleteMetrics(node.ID.String()); err != nil {
		logger.Log(1, "unable to remove metrics from DB for node", node.ID.String(), err.Error())
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
		_, err := GetNetworkByNode(node)
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
		nodesMap[node.ID.String()] = node
	}

	return nodes, nil
}

// GetNetworkByNode - gets the network model from a node
func GetNetworkByNode(node *models.Node) (models.Network, error) {

	var network = models.Network{}
	networkData, err := database.FetchRecord(database.NETWORKS_TABLE_NAME, node.Network)
	if err != nil {
		return network, err
	}
	if err = json.Unmarshal([]byte(networkData), &network); err != nil {
		return models.Network{}, err
	}
	return network, nil
}

// SetNodeDefaults - sets the defaults of a node to avoid empty fields
func SetNodeDefaults(node *models.Node) {

	parentNetwork, _ := GetNetworkByNode(node)
	_, cidr, err := net.ParseCIDR(parentNetwork.AddressRange)
	if err == nil {
		node.NetworkRange = *cidr
	}
	_, cidr, err = net.ParseCIDR(parentNetwork.AddressRange6)
	if err == nil {
		node.NetworkRange6 = *cidr
	}

	if node.DefaultACL == "" {
		node.DefaultACL = parentNetwork.DefaultACL
	}
	if node.FailOverPeers == nil {
		node.FailOverPeers = make(map[string]struct{})
	}

	node.SetLastModified()
	node.SetLastCheckIn()
	node.SetDefaultConnected()
	node.SetExpirationDateTime()
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

	SetNodeDefaults(&node)

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
		newApiNode := nodes[i].ConvertToAPINode()
		apiNodes = append(apiNodes, *newApiNode)
	}
	return apiNodes[:]
}

// DeleteExpiredNodes - goroutine which deletes nodes which are expired
func DeleteExpiredNodes(ctx context.Context, peerUpdate chan *models.Node) {
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
					peerUpdate <- &node
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

	host, err := GetHost(node.HostID.String())
	if err != nil {
		return err
	}

	if !node.DNSOn {
		if servercfg.IsDNSMode() {
			node.DNSOn = true
		} else {
			node.DNSOn = false
		}
	}

	SetNodeDefaults(node)

	defaultACLVal := acls.Allowed
	parentNetwork, err := GetNetwork(node.Network)
	if err == nil {
		if parentNetwork.DefaultACL != "yes" {
			defaultACLVal = acls.NotAllowed
		}
	}

	if node.DefaultACL == "" {
		node.DefaultACL = "unset"
	}

	if node.Address.IP == nil {
		if parentNetwork.IsIPv4 == "yes" {
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
		return fmt.Errorf("invalid address: ipv4 " + node.Address.String() + " is not unique")
	}
	if node.Address6.IP == nil {
		if parentNetwork.IsIPv6 == "yes" {
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
		return fmt.Errorf("invalid address: ipv6 " + node.Address6.String() + " is not unique")
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
	}
	_, err = nodeacls.CreateNodeACL(nodeacls.NetworkID(node.Network), nodeacls.NodeID(node.ID.String()), defaultACLVal)
	if err != nil {
		logger.Log(1, "failed to create node ACL for node,", node.ID.String(), "err:", err.Error())
		return err
	}

	if err = UpdateProNodeACLs(node); err != nil {
		logger.Log(1, "failed to apply node level ACLs during creation of node", node.ID.String(), "-", err.Error())
		return err
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

func ValidateEgressRange(gateway models.EgressGatewayRequest) error {
	network, err := GetNetworkSettings(gateway.NetID)
	if err != nil {
		slog.Error("error getting network with netid", "error", gateway.NetID, err.Error)
		return errors.New("error getting network with netid:  " + gateway.NetID + " " + err.Error())
	}
	ipv4Net := network.AddressRange
	ipv6Net := network.AddressRange6

	for _, v := range gateway.Ranges {
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
