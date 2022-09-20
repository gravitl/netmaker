package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/go-playground/validator/v10"
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
	"golang.org/x/crypto/bcrypt"
)

// RELAY_NODE_ERR - error to return if relay node is unfound
const RELAY_NODE_ERR = "could not find relay for node"

// GetNetworkNodes - gets the nodes of a network
func GetNetworkNodes(network string) ([]models.Node, error) {
	var nodes []models.Node
	allnodes, err := GetAllNodes()
	if err != nil {
		return []models.Node{}, err
	}
	for _, node := range allnodes {
		if node.Network == network {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

// GetSortedNetworkServerNodes - gets nodes of a network, except sorted by update time
func GetSortedNetworkServerNodes(network string) ([]models.Node, error) {
	var nodes []models.Node
	collection, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		if database.IsEmptyRecord(err) {
			return []models.Node{}, nil
		}
		return nodes, err
	}
	for _, value := range collection {

		var node models.Node
		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			continue
		}
		if node.Network == network && node.IsServer == "yes" {
			nodes = append(nodes, node)
		}
	}
	sort.Sort(models.NodesArray(nodes))
	return nodes, nil
}

// GetServerNodes - gets the server nodes of a network
func GetServerNodes(network string) []models.Node {
	var serverNodes = make([]models.Node, 0)
	var nodes, err = GetNetworkNodes(network)
	if err != nil {
		return serverNodes
	}
	for _, node := range nodes {
		if node.IsServer == "yes" {
			serverNodes = append(serverNodes, node)
		}
	}
	return serverNodes
}

// UncordonNode - approves a node to join a network
func UncordonNode(nodeid string) (models.Node, error) {
	node, err := GetNodeByID(nodeid)
	if err != nil {
		return models.Node{}, err
	}
	node.SetLastModified()
	node.IsPending = "no"
	data, err := json.Marshal(&node)
	if err != nil {
		return node, err
	}

	err = database.Insert(node.ID, string(data), database.NODES_TABLE_NAME)
	return node, err
}

// SetIfLeader - gets the peers of a given server node
func SetPeersIfLeader(node *models.Node) {
	if IsLeader(node) {
		setNetworkServerPeers(node)
	}
}

// IsLeader - determines if a given server node is a leader
func IsLeader(node *models.Node) bool {
	nodes, err := GetSortedNetworkServerNodes(node.Network)
	if err != nil {
		logger.Log(0, "ERROR: COULD NOT RETRIEVE SERVER NODES. THIS WILL BREAK HOLE PUNCHING.")
		return false
	}
	for _, n := range nodes {
		if n.LastModified > time.Now().Add(-1*time.Minute).Unix() {
			return n.Address == node.Address
		}
	}
	return len(nodes) <= 1 || nodes[1].Address == node.Address
}

// == DB related functions ==

// UpdateNode - takes a node and updates another node with it's values
func UpdateNode(currentNode *models.Node, newNode *models.Node) error {
	if newNode.Address != currentNode.Address {
		if network, err := GetParentNetwork(newNode.Network); err == nil {
			if !IsAddressInCIDR(newNode.Address, network.AddressRange) {
				return fmt.Errorf("invalid address provided; out of network range for node %s", newNode.ID)
			}
		}
	}
	nodeACLDelta := currentNode.DefaultACL != newNode.DefaultACL
	newNode.Fill(currentNode)

	if currentNode.IsServer == "yes" && !validateServer(currentNode, newNode) {
		return fmt.Errorf("this operation is not supported on server nodes")
	}
	// check for un-settable server values
	if err := ValidateNode(newNode, true); err != nil {
		return err
	}

	if newNode.ID == currentNode.ID {
		if nodeACLDelta {
			if err := updateProNodeACLS(newNode); err != nil {
				logger.Log(1, "failed to apply node level ACLs during creation of node", newNode.ID, "-", err.Error())
				return err
			}
		}

		newNode.SetLastModified()
		if data, err := json.Marshal(newNode); err != nil {
			return err
		} else {
			return database.Insert(newNode.ID, string(data), database.NODES_TABLE_NAME)
		}
	}

	return fmt.Errorf("failed to update node " + currentNode.ID + ", cannot change ID.")
}

// DeleteNodeByID - deletes a node from database or moves into delete nodes table
func DeleteNodeByID(node *models.Node, exterminate bool) error {
	var err error
	var key = node.ID
	//delete any ext clients as required
	if node.IsIngressGateway == "yes" {
		if err := DeleteGatewayExtClients(node.ID, node.Network); err != nil {
			logger.Log(0, "failed to deleted ext clients", err.Error())
		}
	}
	if !exterminate {
		node.Action = models.NODE_DELETE
		nodedata, err := json.Marshal(&node)
		if err != nil {
			return err
		}
		err = database.Insert(key, string(nodedata), database.DELETED_NODES_TABLE_NAME)
		if err != nil {
			return err
		}
	} else {
		if err := database.DeleteRecord(database.DELETED_NODES_TABLE_NAME, key); err != nil {
			logger.Log(2, err.Error())
		}
	}
	if err = database.DeleteRecord(database.NODES_TABLE_NAME, key); err != nil {
		return err
	}

	if servercfg.IsDNSMode() {
		SetDNS()
	}
	if node.OwnerID != "" {
		err = pro.DissociateNetworkUserNode(node.OwnerID, node.Network, node.ID)
		if err != nil {
			logger.Log(0, "failed to dissasociate", node.OwnerID, "from node", node.ID, ":", err.Error())
		}
	}

	_, err = nodeacls.RemoveNodeACL(nodeacls.NetworkID(node.Network), nodeacls.NodeID(node.ID))
	if err != nil {
		// ignoring for now, could hit a nil pointer if delete called twice
		logger.Log(2, "attempted to remove node ACL for node", node.Name, node.ID)
	}
	// removeZombie <- node.ID
	if err = DeleteMetrics(node.ID); err != nil {
		logger.Log(1, "unable to remove metrics from DB for node", node.ID, err.Error())
	}

	if node.IsServer == "yes" {
		return removeLocalServer(node)
	}
	return nil
}

// IsNodeIDUnique - checks if node id is unique
func IsNodeIDUnique(node *models.Node) (bool, error) {
	_, err := database.FetchRecord(database.NODES_TABLE_NAME, node.ID)
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
	_ = v.RegisterValidation("in_charset", func(fl validator.FieldLevel) bool {
		isgood := node.NameInNodeCharSet()
		return isgood
	})
	_ = v.RegisterValidation("checkyesorno", func(fl validator.FieldLevel) bool {
		return validation.CheckYesOrNo(fl)
	})
	_ = v.RegisterValidation("checkyesornoorunset", func(fl validator.FieldLevel) bool {
		return validation.CheckYesOrNoOrUnset(fl)
	})
	err := v.Struct(node)

	return err
}

// CreateNode - creates a node in database
func CreateNode(node *models.Node) error {

	//encrypt that password so we never see it
	hash, err := bcrypt.GenerateFromPassword([]byte(node.Password), 5)
	if err != nil {
		return err
	}
	//set password to encrypted password
	node.Password = string(hash)
	if node.Name == models.NODE_SERVER_NAME {
		node.IsServer = "yes"
	}
	if node.DNSOn == "" {
		if servercfg.IsDNSMode() {
			node.DNSOn = "yes"
		} else {
			node.DNSOn = "no"
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

	reverse := node.IsServer == "yes"
	if node.Address == "" {
		if parentNetwork.IsIPv4 == "yes" {
			if node.Address, err = UniqueAddress(node.Network, reverse); err != nil {
				return err
			}
		}
	} else if !IsIPUnique(node.Network, node.Address, database.NODES_TABLE_NAME, false) {
		return fmt.Errorf("invalid address: ipv4 " + node.Address + " is not unique")
	}

	if node.Address6 == "" {
		if parentNetwork.IsIPv6 == "yes" {
			if node.Address6, err = UniqueAddress6(node.Network, reverse); err != nil {
				return err
			}
		}
	} else if !IsIPUnique(node.Network, node.Address6, database.NODES_TABLE_NAME, true) {
		return fmt.Errorf("invalid address: ipv6 " + node.Address6 + " is not unique")
	}

	node.ID = uuid.NewString()

	//Create a JWT for the node
	tokenString, _ := CreateJWT(node.ID, node.MacAddress, node.Network)
	if tokenString == "" {
		//logic.ReturnErrorResponse(w, r, errorResponse)
		return err
	}
	err = ValidateNode(node, false)
	if err != nil {
		return err
	}
	// CheckZombies(node)

	nodebytes, err := json.Marshal(&node)
	if err != nil {
		return err
	}
	err = database.Insert(node.ID, string(nodebytes), database.NODES_TABLE_NAME)
	if err != nil {
		return err
	}

	_, err = nodeacls.CreateNodeACL(nodeacls.NetworkID(node.Network), nodeacls.NodeID(node.ID), defaultACLVal)
	if err != nil {
		logger.Log(1, "failed to create node ACL for node,", node.ID, "err:", err.Error())
		return err
	}

	if err = updateProNodeACLS(node); err != nil {
		logger.Log(1, "failed to apply node level ACLs during creation of node", node.ID, "-", err.Error())
		return err
	}

	if node.IsPending != "yes" {
		DecrimentKey(node.Network, node.AccessKey)
	}

	if err = UpdateMetrics(node.ID, &models.Metrics{Connectivity: make(map[string]models.Metric)}); err != nil {
		logger.Log(1, "failed to initialize metrics for node", node.Name, node.ID, err.Error())
	}

	SetNetworkNodesLastModified(node.Network)
	if servercfg.IsDNSMode() {
		err = SetDNS()
	}
	return err
}

// GetAllNodes - returns all nodes in the DB
func GetAllNodes() ([]models.Node, error) {
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
		if err := json.Unmarshal([]byte(value), &node); err != nil {
			return []models.Node{}, err
		}
		// add node to our array
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// CheckIsServer - check if a node is the server node
func CheckIsServer(node *models.Node) bool {
	nodeData, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil && !database.IsEmptyRecord(err) {
		return false
	}
	for _, value := range nodeData {
		var tmpNode models.Node
		if err := json.Unmarshal([]byte(value), &tmpNode); err != nil {
			continue
		}
		if tmpNode.Network == node.Network && tmpNode.MacAddress != node.MacAddress {
			return false
		}
	}
	return true
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

	//TODO: Maybe I should make Network a part of the node struct. Then we can just query the Network object for stuff.
	parentNetwork, _ := GetNetworkByNode(node)
	node.NetworkSettings = parentNetwork
	node.NetworkSettings.AccessKeys = []models.AccessKey{}

	node.ExpirationDateTime = time.Now().Unix() + models.TEN_YEARS_IN_SECONDS

	if node.DefaultACL == "" && node.IsServer != "yes" {
		node.DefaultACL = parentNetwork.DefaultACL
	}

	if node.ListenPort == 0 {
		node.ListenPort = parentNetwork.DefaultListenPort
	}

	if node.Interface == "" {
		node.Interface = parentNetwork.DefaultInterface
	}
	if node.PersistentKeepalive == 0 {
		node.PersistentKeepalive = parentNetwork.DefaultKeepalive
	}
	if node.PostUp == "" {
		postup := parentNetwork.DefaultPostUp
		node.PostUp = postup
	}
	if node.PostDown == "" {
		postdown := parentNetwork.DefaultPostDown
		node.PostDown = postdown
	}
	if node.IsStatic == "" {
		node.IsStatic = "no"
	}
	if node.UDPHolePunch == "" {
		node.UDPHolePunch = parentNetwork.DefaultUDPHolePunch
		if node.UDPHolePunch == "" {
			node.UDPHolePunch = "yes"
		}
	}
	// == Parent Network settings ==

	if node.MTU == 0 {
		node.MTU = parentNetwork.DefaultMTU
	}
	// == node defaults if not set by parent ==
	node.SetIPForwardingDefault()
	node.SetDNSOnDefault()
	node.SetIsLocalDefault()
	node.SetLastModified()
	node.SetDefaultName()
	node.SetLastCheckIn()
	node.SetLastPeerUpdate()
	node.SetDefaultAction()
	node.SetIsServerDefault()
	node.SetIsStaticDefault()
	node.SetDefaultEgressGateway()
	node.SetDefaultIngressGateway()
	node.SetDefaulIsPending()
	node.SetDefaultMTU()
	node.SetDefaultNFTablesPresent()
	node.SetDefaultIsRelayed()
	node.SetDefaultIsRelay()
	node.SetDefaultIsDocker()
	node.SetDefaultIsK8S()
	node.SetDefaultIsHub()
	node.SetDefaultConnected()
	node.SetDefaultACL()
}

// GetRecordKey - get record key
// depricated
func GetRecordKey(id string, network string) (string, error) {
	if id == "" || network == "" {
		return "", errors.New("unable to get record key")
	}
	return id + "###" + network, nil
}

// GetNodeByMacAddress - gets a node by mac address
func GetNodeByMacAddress(network string, macaddress string) (models.Node, error) {

	var node models.Node

	key, err := GetRecordKey(macaddress, network)
	if err != nil {
		return node, err
	}

	record, err := database.FetchRecord(database.NODES_TABLE_NAME, key)
	if err != nil {
		return models.Node{}, err
	}

	if err = json.Unmarshal([]byte(record), &node); err != nil {
		return models.Node{}, err
	}

	SetNodeDefaults(&node)

	return node, nil
}

// GetNodesByAddress - gets a node by mac address
func GetNodesByAddress(network string, addresses []string) ([]models.Node, error) {
	var nodes []models.Node
	allnodes, err := GetAllNodes()
	if err != nil {
		return []models.Node{}, err
	}
	for _, node := range allnodes {
		if node.Network == network && ncutils.StringSliceContains(addresses, node.Address) {
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
		if relay.IsRelay == "yes" {
			for _, addr := range relay.RelayAddrs {
				if addr == relayedNodeAddr {
					return relay, nil
				}
			}
		}
	}
	return relay, errors.New(RELAY_NODE_ERR + " " + relayedNodeAddr)
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

// GetNetworkServerNodeID - get network server node ID if exists
func GetNetworkServerLeader(network string) (models.Node, error) {
	nodes, err := GetSortedNetworkServerNodes(network)
	if err != nil {
		return models.Node{}, err
	}
	for _, node := range nodes {
		if IsLeader(&node) {
			return node, nil
		}
	}
	return models.Node{}, errors.New("could not find server leader")
}

// GetNetworkServerNodeID - get network server node ID if exists
func GetNetworkServerLocal(network string) (models.Node, error) {
	nodes, err := GetSortedNetworkServerNodes(network)
	if err != nil {
		return models.Node{}, err
	}
	mac := servercfg.GetNodeID()
	if mac == "" {
		return models.Node{}, fmt.Errorf("error retrieving local server node: server node ID is unset")
	}
	for _, node := range nodes {
		if mac == node.MacAddress {
			return node, nil
		}
	}
	return models.Node{}, errors.New("could not find node for local server")
}

// IsLocalServer - get network server node ID if exists
func IsLocalServer(node *models.Node) bool {
	var islocal bool
	local, err := GetNetworkServerLocal(node.Network)
	if err != nil {
		return islocal
	}
	return node.ID != "" && local.ID == node.ID
}

// validateServer - make sure servers dont change port or address
func validateServer(currentNode, newNode *models.Node) bool {
	return (newNode.Address == currentNode.Address &&
		newNode.ListenPort == currentNode.ListenPort &&
		newNode.IsServer == "yes")
}

// unsetHub - unset hub on network nodes
func UnsetHub(networkName string) (*models.Node, error) {
	var nodesToUpdate models.Node
	nodes, err := GetNetworkNodes(networkName)
	if err != nil {
		return &nodesToUpdate, err
	}

	for i := range nodes {
		if nodes[i].IsHub == "yes" {
			nodes[i].IsHub = "no"
			nodesToUpdate = nodes[i]
			newNodeData, err := json.Marshal(&nodes[i])
			if err != nil {
				logger.Log(1, "error on node during hub update")
				return &nodesToUpdate, err
			}
			database.Insert(nodes[i].ID, string(newNodeData), database.NODES_TABLE_NAME)
		}
	}
	return &nodesToUpdate, nil
}

// FindRelay - returns the node that is the relay for a relayed node
func FindRelay(node *models.Node) *models.Node {
	if node.IsRelayed == "no" {
		return nil
	}
	peers, err := GetNetworkNodes(node.Network)
	if err != nil {
		return nil
	}
	for _, peer := range peers {
		if peer.IsRelay == "no" {
			continue
		}
		for _, ip := range peer.RelayAddrs {
			if ip == node.Address || ip == node.Address6 {
				return &peer
			}
		}
	}
	return nil
}

func findNode(ip string) (*models.Node, error) {
	nodes, err := GetAllNodes()
	if err != nil {
		return nil, err
	}
	for _, node := range nodes {
		if node.Address == ip {
			return &node, nil
		}
		if node.Address6 == ip {
			return &node, nil
		}
	}
	return nil, errors.New("node not found")
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

// == END PRO ==
