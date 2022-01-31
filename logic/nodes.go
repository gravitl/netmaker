package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/validation"
	"golang.org/x/crypto/bcrypt"
)

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
	node.PullChanges = "yes"
	data, err := json.Marshal(&node)
	if err != nil {
		return node, err
	}

	err = database.Insert(node.ID, string(data), database.NODES_TABLE_NAME)
	return node, err
}

// GetPeers - gets the peers of a given server node
func GetPeers(node *models.Node) ([]models.Node, error) {
	if IsLeader(node) {
		setNetworkServerPeers(node)
	}
	excludeIsRelayed := node.IsRelay != "yes"
	var relayedNode string
	if node.IsRelayed == "yes" {
		relayedNode = node.Address
	}
	peers, err := GetPeersList(node.Network, excludeIsRelayed, relayedNode)
	if err != nil {
		return nil, err
	}
	return peers, nil
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
	newNode.Fill(currentNode)
	if err := ValidateNode(newNode, true); err != nil {
		return err
	}
	if newNode.ID == currentNode.ID {
		newNode.SetLastModified()
		if data, err := json.Marshal(newNode); err != nil {
			return err
		} else {
			return database.Insert(newNode.ID, string(data), database.NODES_TABLE_NAME)
		}
	}
	return fmt.Errorf("failed to update node " + newNode.MacAddress + ", cannot change macaddress.")
}

// IsNodeIDUnique - checks if node id is unique
func IsNodeIDUnique(node *models.Node) (bool, error) {
	_, err := database.FetchRecord(database.NODES_TABLE_NAME, node.ID)
	return database.IsEmptyRecord(err), err
}

// ValidateNode - validates node values
func ValidateNode(node *models.Node, isUpdate bool) error {
	v := validator.New()
	_ = v.RegisterValidation("macaddress_unique", func(fl validator.FieldLevel) bool {
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
	node.Address, err = UniqueAddress(node.Network)
	if err != nil {
		return err
	}
	node.Address6, err = UniqueAddress6(node.Network)
	if err != nil {
		return err
	}

	// TODO: This covers legacy nodes, eventually want to remove legacy check
	if node.IsServer == "yes" {
		node.ID = uuid.NewString()
	} else if node.IsServer != "yes" || (node.ID == "" || strings.Contains(node.ID, "###")) {
		node.ID = uuid.NewString()
	}

	//Create a JWT for the node
	tokenString, _ := CreateJWT(node.ID, node.MacAddress, node.Network)
	if tokenString == "" {
		//returnErrorResponse(w, r, errorResponse)
		return err
	}
	err = ValidateNode(node, false)
	if err != nil {
		return err
	}

	nodebytes, err := json.Marshal(&node)
	if err != nil {
		return err
	}
	err = database.Insert(node.ID, string(nodebytes), database.NODES_TABLE_NAME)
	if err != nil {
		return err
	}
	if node.IsPending != "yes" {
		DecrimentKey(node.Network, node.AccessKey)
	}
	SetNetworkNodesLastModified(node.Network)
	if servercfg.IsDNSMode() {
		err = SetDNS()
	}
	return err
}

// IfaceDelta - is there interface changes
func IfaceDelta(currentNode *models.Node, newNode *models.Node) bool {
	SetNodeDefaults(newNode)
	// single comparison statements
	if currentNode.IsServer != "yes" {
		return false
	}

	if newNode.Endpoint != currentNode.Endpoint ||
		newNode.LocalAddress != currentNode.LocalAddress ||
		newNode.PublicKey != currentNode.PublicKey ||
		newNode.Address != currentNode.Address ||
		newNode.IsEgressGateway != currentNode.IsEgressGateway ||
		newNode.IsIngressGateway != currentNode.IsIngressGateway ||
		newNode.IsRelay != currentNode.IsRelay ||
		newNode.UDPHolePunch != currentNode.UDPHolePunch ||
		newNode.IsPending != currentNode.IsPending ||
		newNode.PersistentKeepalive != currentNode.PersistentKeepalive ||
		len(newNode.ExcludedAddrs) != len(currentNode.ExcludedAddrs) ||
		len(newNode.AllowedIPs) != len(currentNode.AllowedIPs) {
		return true
	}

	// multi-comparison statements
	if newNode.IsDualStack == "yes" {
		if newNode.Address6 != currentNode.Address6 {
			return true
		}
	}

	if newNode.IsEgressGateway == "yes" {
		if len(currentNode.EgressGatewayRanges) != len(newNode.EgressGatewayRanges) {
			return true
		}
		for _, address := range newNode.EgressGatewayRanges {
			if !StringSliceContains(currentNode.EgressGatewayRanges, address) {
				return true
			}
		}
	}

	if newNode.IsRelay == "yes" {
		if len(currentNode.RelayAddrs) != len(newNode.RelayAddrs) {
			return true
		}
		for _, address := range newNode.RelayAddrs {
			if !StringSliceContains(currentNode.RelayAddrs, address) {
				return true
			}
		}
	}

	for _, address := range newNode.AllowedIPs {
		if !StringSliceContains(currentNode.AllowedIPs, address) {
			return true
		}
	}

	return false
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

	node.ExpirationDateTime = time.Now().Unix() + models.TEN_YEARS_IN_SECONDS

	if node.ListenPort == 0 {
		node.ListenPort = parentNetwork.DefaultListenPort
	}
	if node.SaveConfig == "" {
		if parentNetwork.DefaultSaveConfig != "" {
			node.SaveConfig = parentNetwork.DefaultSaveConfig
		} else {
			node.SaveConfig = "yes"
		}
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
	if node.IsDualStack == "" {
		node.IsDualStack = parentNetwork.IsDualStack
	}
	if node.MTU == 0 {
		node.MTU = parentNetwork.DefaultMTU
	}
	// == node defaults if not set by parent ==
	node.SetIPForwardingDefault()
	node.SetDNSOnDefault()
	node.SetIsLocalDefault()
	node.SetIsDualStackDefault()
	node.SetLastModified()
	node.SetDefaultName()
	node.SetLastCheckIn()
	node.SetLastPeerUpdate()
	node.SetRoamingDefault()
	node.SetPullChangesDefault()
	node.SetDefaultAction()
	node.SetIsServerDefault()
	node.SetIsStaticDefault()
	node.SetDefaultEgressGateway()
	node.SetDefaultIngressGateway()
	node.SetDefaulIsPending()
	node.SetDefaultMTU()
	node.SetDefaultIsRelayed()
	node.SetDefaultIsRelay()
	node.SetDefaultIsDocker()
	node.SetDefaultIsK8S()
	node.KeyUpdateTimeStamp = time.Now().Unix()
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
	return relay, errors.New("could not find relay for node " + relayedNodeAddr)
}

// GetNodeByIDorMacAddress - gets the node, if a mac address exists, but not id, then it should delete it and recreate in DB with new ID
func GetNodeByIDorMacAddress(uuid string, macaddress string, network string) (models.Node, error) {
	var node models.Node
	var err error
	node, err = GetNodeByID(uuid)
	if err != nil && macaddress != "" && network != "" {
		node, err = GetNodeByMacAddress(network, macaddress)
		if err != nil {
			return models.Node{}, err
		}
		err = DeleteNodeByMacAddress(&node, true) // remove node
		if err != nil {
			return models.Node{}, err
		}
		err = CreateNode(&node)
		if err != nil {
			return models.Node{}, err
		}
		logger.Log(2, "rewriting legacy node data; node now has id,", node.ID)
		node.PullChanges = "yes"
	}
	return node, err
}

// GetNodeByID - get node by uuid, should have been set by create
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
func GetNetworkServerNodeID(network string) (string, error) {
	var nodes, err = GetNetworkNodes(network)
	if err != nil {
		return "", err
	}
	for _, node := range nodes {
		if node.IsServer == "yes" {
			if servercfg.GetNodeID() != "" {
				if servercfg.GetNodeID() == node.MacAddress {
					if strings.Contains(node.ID, "###") {
						DeleteNodeByMacAddress(&node, true)
						logger.Log(1, "deleted legacy server node on network "+node.Network)
						return "", errors.New("deleted legacy server node on network " + node.Network)
					}
					return node.ID, nil
				}
				continue
			}
			return node.ID, nil
		}
	}
	return "", errors.New("could not find server node")
}
