package logic

import (
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
	"github.com/gravitl/netmaker/logic/pro"
	"github.com/gravitl/netmaker/logic/pro/proacls"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/validation"
)

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
	return database.Insert(node.ID.String(), string(data), database.NODES_TABLE_NAME)
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
	newNode.Fill(currentNode)

	// check for un-settable server values
	if err := ValidateNode(newNode, true); err != nil {
		return err
	}

	if newNode.ID == currentNode.ID {
		if nodeACLDelta {
			if err := updateProNodeACLS(newNode); err != nil {
				logger.Log(1, "failed to apply node level ACLs during creation of node", newNode.ID.String(), "-", err.Error())
				return err
			}
		}

		newNode.SetLastModified()
		if data, err := json.Marshal(newNode); err != nil {
			return err
		} else {
			// invalidate cache
			CacheNodesMutex.Lock()
			CacheNodes = nil
			CacheNodesMutex.Unlock()
			return database.Insert(newNode.ID.String(), string(data), database.NODES_TABLE_NAME)
		}
	}

	return fmt.Errorf("failed to update node " + currentNode.ID.String() + ", cannot change ID.")
}

// DeleteNode - marks node for deletion (and adds to zombie list) if called by UI or deletes node if called by node
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
	host, err := GetHost(node.HostID.String())
	if err != nil {
		logger.Log(1, "no host found for node", node.ID.String(), "deleting..")
		if delErr := deleteNodeByID(node); delErr != nil {
			logger.Log(0, "failed to delete node", node.ID.String(), delErr.Error())
		}
		return err
	}
	if err := DissasociateNodeFromHost(node, host); err != nil {
		return err
	}
	if servercfg.Is_EE {
		if err := EnterpriseResetAllPeersFailovers(node.ID, node.Network); err != nil {
			logger.Log(0, "failed to reset failover lists during node delete for node", host.Name, node.Network)
		}
	}

	return nil
}

// deleteNodeByID - deletes a node from database
func deleteNodeByID(node *models.Node) error {
	var err error
	var key = node.ID.String()
	//delete any ext clients as required
	if node.IsIngressGateway {
		if err := DeleteGatewayExtClients(node.ID.String(), node.Network); err != nil {
			logger.Log(0, "failed to deleted ext clients", err.Error())
		}
	}
	// invalidate cache
	CacheNodesMutex.Lock()
	CacheNodes = nil
	CacheNodesMutex.Unlock()
	if err = database.DeleteRecord(database.NODES_TABLE_NAME, key); err != nil {
		if !database.IsEmptyRecord(err) {
			return err
		}
	}
	if servercfg.IsDNSMode() {
		SetDNS()
	}
	if node.OwnerID != "" {
		err = pro.DissociateNetworkUserNode(node.OwnerID, node.Network, node.ID.String())
		if err != nil {
			logger.Log(0, "failed to dissasociate", node.OwnerID, "from node", node.ID.String(), ":", err.Error())
		}
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

// IsFailoverPresent - checks if a node is marked as a failover in given network
func IsFailoverPresent(network string) bool {
	netNodes, err := GetNetworkNodes(network)
	if err != nil {
		return false
	}
	for i := range netNodes {
		if netNodes[i].Failover {
			return true
		}
	}
	return false
}

var CacheNodes []models.Node
var CacheNodesMutex = sync.RWMutex{}

// GetAllNodes - returns all nodes in the DB
func GetAllNodes() ([]models.Node, error) {
	CacheNodesMutex.RLock()
	if CacheNodes != nil {
		defer CacheNodesMutex.RUnlock()
		return CacheNodes, nil
	}
	CacheNodesMutex.RUnlock()
	var nodes []models.Node

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
	}

	CacheNodesMutex.Lock()
	CacheNodes = nodes
	CacheNodesMutex.Unlock()

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

	if node.PersistentKeepalive == 0 {
		node.PersistentKeepalive = time.Second * time.Duration(parentNetwork.DefaultKeepalive)
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

// GetNodesByAddress - gets a node by mac address
func GetNodesByAddress(network string, addresses []string) ([]models.Node, error) {
	var nodes []models.Node
	allnodes, err := GetAllNodes()
	if err != nil {
		return []models.Node{}, err
	}
	for _, node := range allnodes {
		if node.Network == network && ncutils.StringSliceContains(addresses, node.Address.String()) {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

// GetDeletedNodeByMacAddress - get a deleted node
func GetDeletedNodeByMacAddress(network string, macaddress string) (models.Node, error) {

	var node models.Node

	key, err := GetRecordKey(macaddress, network)
	if err != nil {
		return node, err
	}

	record, err := database.FetchRecord(database.DELETED_NODES_TABLE_NAME, key)
	if err != nil {
		return models.Node{}, err
	}

	if err = json.Unmarshal([]byte(record), &node); err != nil {
		return models.Node{}, err
	}

	SetNodeDefaults(&node)

	return node, nil
}

// GetNodeRelay - gets the relay node of a given network
func GetNodeRelay(network string, relayedNodeAddr string) (models.Node, error) {
	collection, err := database.FetchRecords(database.NODES_TABLE_NAME)
	var relay models.Node
	if err != nil {
		if database.IsEmptyRecord(err) {
			return relay, nil
		}
		logger.Log(2, err.Error())
		return relay, err
	}
	for _, value := range collection {
		err := json.Unmarshal([]byte(value), &relay)
		if err != nil {
			logger.Log(2, err.Error())
			continue
		}
		if relay.IsRelay {
			for _, addr := range relay.RelayAddrs {
				if addr == relayedNodeAddr {
					return relay, nil
				}
			}
		}
	}
	return relay, errors.New(RELAY_NODE_ERR + " " + relayedNodeAddr)
}

// TODO pointer
func GetNodeByID(uuid string) (models.Node, error) {
	CacheNodesMutex.RLock()
	for _, node := range CacheNodes {
		if node.ID.String() == uuid {
			defer CacheNodesMutex.RUnlock()
			return node, nil
		}
	}
	CacheNodesMutex.RUnlock()
	var record, err = database.FetchRecord(database.NODES_TABLE_NAME, uuid)
	if err != nil {
		return models.Node{}, err
	}
	var node models.Node
	if err = json.Unmarshal([]byte(record), &node); err != nil {
		return models.Node{}, err
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
	if !node.IsRelayed {
		return nil
	}
	peers, err := GetNetworkNodes(node.Network)
	if err != nil {
		return nil
	}
	for _, peer := range peers {
		if !peer.IsRelay {
			continue
		}
		for _, ip := range peer.RelayAddrs {
			if ip == node.Address.IP.String() || ip == node.Address6.IP.String() {
				return &peer
			}
		}
	}
	return nil
}

// GetNetworkIngresses - gets the gateways of a network
func GetNetworkIngresses(network string) ([]models.Node, error) {
	var ingresses []models.Node
	netNodes, err := GetNetworkNodes(network)
	if err != nil {
		return []models.Node{}, err
	}
	for i := range netNodes {
		if netNodes[i].IsIngressGateway {
			ingresses = append(ingresses, netNodes[i])
		}
	}
	return ingresses, nil
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

// == PRO ==

func updateProNodeACLS(node *models.Node) error {
	// == PRO node ACLs ==
	networkNodes, err := GetNetworkNodes(node.Network)
	if err != nil {
		return err
	}
	if err = proacls.AdjustNodeAcls(node, networkNodes[:]); err != nil {
		return err
	}
	return nil
}

// createNode - creates a node in database
func createNode(node *models.Node) error {
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
	// invalidate cache
	CacheNodesMutex.Lock()
	CacheNodes = nil
	CacheNodesMutex.Unlock()
	err = database.Insert(node.ID.String(), string(nodebytes), database.NODES_TABLE_NAME)
	if err != nil {
		return err
	}

	_, err = nodeacls.CreateNodeACL(nodeacls.NetworkID(node.Network), nodeacls.NodeID(node.ID.String()), defaultACLVal)
	if err != nil {
		logger.Log(1, "failed to create node ACL for node,", node.ID.String(), "err:", err.Error())
		return err
	}

	if err = updateProNodeACLS(node); err != nil {
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

// == END PRO ==
