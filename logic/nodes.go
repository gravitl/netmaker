package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gravitl/netmaker/converters"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/gorm"
	"net"
	"sort"
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

const (
	// RELAY_NODE_ERR - error to return if relay node is unfound
	RELAY_NODE_ERR = "could not find relay for node"
	// NodePurgeTime time to wait for node to response to a NODE_DELETE actions
	NodePurgeTime = time.Second * 10
	// NodePurgeCheckTime is how often to check nodes for Pending Delete
	NodePurgeCheckTime = time.Second * 30
)

// GetNetworkNodes - gets the nodes of a network
func GetNetworkNodes(networkID string) ([]models.Node, error) {
	_network := &schema.Network{
		ID: networkID,
	}
	err := _network.GetNodes(db.WithContext(context.TODO()))
	if err != nil {
		return nil, err
	}

	return converters.ToModelNodes(_network.Nodes), nil
}

// UpdateNodeCheckin - updates the checkin time of a node
func UpdateNodeCheckin(node *models.Node) error {
	_node := &schema.Node{
		ID:          node.ID.String(),
		LastCheckIn: time.Now(),
	}
	return _node.UpdateLastCheckIn(db.WithContext(context.TODO()))
}

// UpsertNode - updates node in the DB
func UpsertNode(newNode *models.Node) error {
	newNode.SetLastModified()

	_node := converters.ToSchemaNode(*newNode)
	return _node.Upsert(db.WithContext(context.TODO()))
}

// UpdateNode - takes a node and updates another node with it's values
func UpdateNode(currentNode *models.Node, newNode *models.Node) error {
	if newNode.Address.IP.String() != currentNode.Address.IP.String() {
		if network, err := GetNetwork(newNode.Network); err == nil {
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
		_node := converters.ToSchemaNode(*newNode)
		return _node.Update(db.WithContext(context.TODO()))
	}

	return fmt.Errorf("failed to update node %s, cannot change ID", currentNode.ID.String())
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
	go RemoveNodeFromAclPolicy(*node)

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
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
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
	_node := &schema.Node{
		ID: node.ID.String(),
	}
	return _node.Exists(db.WithContext(context.TODO()))
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
		_, err := GetNetwork(node.Network)
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
	_nodes, err := (&schema.Node{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		return nil, err
	}

	return converters.ToModelNodes(_nodes), nil
}

func AddStaticNodestoList(nodes []models.Node) []models.Node {
	netMap := make(map[string]struct{})
	for _, node := range nodes {
		if _, ok := netMap[node.Network]; ok {
			continue
		}
		if node.IsIngressGateway {
			nodes = append(nodes, GetStaticNodesByNetwork(models.NetworkID(node.Network), false)...)
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
			defaultPolicy, _ := GetDefaultPolicy(models.NetworkID(node.Network), models.DevicePolicy)
			aclDefaultPolicyStatusMap[node.Network] = defaultPolicy.Enabled
		}
		if statusCall {
			GetNodeStatus(&node, aclDefaultPolicyStatusMap[node.Network])
		} else {
			GetNodeCheckInStatus(&node, true)
		}

		nodesWithStatus = append(nodesWithStatus, node)
	}
	return
}

// SetNodeDefaults - sets the defaults of a node to avoid empty fields
func SetNodeDefaults(node *models.Node, resetConnected bool) {

	parentNetwork, _ := GetNetwork(node.Network)
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

func GetNodeByID(nodeID string) (models.Node, error) {
	_node := &schema.Node{
		ID: nodeID,
	}
	err := _node.Get(db.WithContext(context.TODO()))
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
		newApiNode := nodes[i].ConvertToAPINode()
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

	SetNodeDefaults(node, true)

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

	_node := converters.ToSchemaNode(*node)
	err = _node.Create(db.WithContext(context.TODO()))
	if err != nil {
		return err
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

func ValidateEgressRange(gateway models.EgressGatewayRequest) error {
	network, err := GetNetwork(gateway.NetID)
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

func GetTagMapWithNodes() (tagNodesMap map[models.TagID][]models.Node) {
	tagNodesMap = make(map[models.TagID][]models.Node)
	nodes, _ := GetAllNodes()
	for _, nodeI := range nodes {
		if nodeI.Tags == nil {
			continue
		}
		if nodeI.Mutex != nil {
			nodeI.Mutex.Lock()
		}
		for nodeTagID := range nodeI.Tags {
			tagNodesMap[nodeTagID] = append(tagNodesMap[nodeTagID], nodeI)
		}
		if nodeI.Mutex != nil {
			nodeI.Mutex.Unlock()
		}

	}
	return
}

func GetTagMapWithNodesByNetwork(netID models.NetworkID, withStaticNodes bool) (tagNodesMap map[models.TagID][]models.Node) {
	tagNodesMap = make(map[models.TagID][]models.Node)
	nodes, _ := GetNetworkNodes(netID.String())
	for _, nodeI := range nodes {
		tagNodesMap[models.TagID(nodeI.ID.String())] = []models.Node{
			nodeI,
		}
		if nodeI.Tags == nil {
			continue
		}
		if nodeI.Mutex != nil {
			nodeI.Mutex.Lock()
		}
		for nodeTagID := range nodeI.Tags {
			if nodeTagID == models.TagID(nodeI.ID.String()) {
				continue
			}
			tagNodesMap[nodeTagID] = append(tagNodesMap[nodeTagID], nodeI)
		}
		if nodeI.Mutex != nil {
			nodeI.Mutex.Unlock()
		}
	}
	tagNodesMap["*"] = nodes
	if !withStaticNodes {
		return
	}
	return AddTagMapWithStaticNodes(netID, tagNodesMap)
}

func AddTagMapWithStaticNodes(netID models.NetworkID,
	tagNodesMap map[models.TagID][]models.Node) map[models.TagID][]models.Node {
	extclients, err := GetNetworkExtClients(netID.String())
	if err != nil {
		return tagNodesMap
	}
	for _, extclient := range extclients {
		if extclient.RemoteAccessClientID != "" {
			continue
		}
		tagNodesMap[models.TagID(extclient.ClientID)] = []models.Node{
			{
				IsStatic:   true,
				StaticNode: extclient,
			},
		}
		if extclient.Tags == nil {
			continue
		}

		if extclient.Mutex != nil {
			extclient.Mutex.Lock()
		}
		for tagID := range extclient.Tags {
			if tagID == models.TagID(extclient.ClientID) {
				continue
			}
			tagNodesMap[tagID] = append(tagNodesMap[tagID], extclient.ConvertToStaticNode())
			tagNodesMap["*"] = append(tagNodesMap["*"], extclient.ConvertToStaticNode())
		}
		if extclient.Mutex != nil {
			extclient.Mutex.Unlock()
		}
	}
	return tagNodesMap
}

func AddTagMapWithStaticNodesWithUsers(netID models.NetworkID,
	tagNodesMap map[models.TagID][]models.Node) map[models.TagID][]models.Node {
	extclients, err := GetNetworkExtClients(netID.String())
	if err != nil {
		return tagNodesMap
	}
	for _, extclient := range extclients {
		tagNodesMap[models.TagID(extclient.ClientID)] = []models.Node{
			{
				IsStatic:   true,
				StaticNode: extclient,
			},
		}
		if extclient.Tags == nil {
			continue
		}
		if extclient.Mutex != nil {
			extclient.Mutex.Lock()
		}
		for tagID := range extclient.Tags {
			tagNodesMap[tagID] = append(tagNodesMap[tagID], extclient.ConvertToStaticNode())
		}
		if extclient.Mutex != nil {
			extclient.Mutex.Unlock()
		}

	}
	return tagNodesMap
}

func GetNodesWithTag(tagID models.TagID) map[string]models.Node {
	nMap := make(map[string]models.Node)
	tag, err := GetTag(tagID)
	if err != nil {
		return nMap
	}
	nodes, _ := GetNetworkNodes(tag.Network.String())
	for _, nodeI := range nodes {
		if nodeI.Tags == nil {
			continue
		}
		if nodeI.Mutex != nil {
			nodeI.Mutex.Lock()
		}
		if _, ok := nodeI.Tags[tagID]; ok {
			nMap[nodeI.ID.String()] = nodeI
		}
		if nodeI.Mutex != nil {
			nodeI.Mutex.Unlock()
		}
	}
	return AddStaticNodesWithTag(tag, nMap)
}

func AddStaticNodesWithTag(tag models.Tag, nMap map[string]models.Node) map[string]models.Node {
	extclients, err := GetNetworkExtClients(tag.Network.String())
	if err != nil {
		return nMap
	}
	for _, extclient := range extclients {
		if extclient.RemoteAccessClientID != "" {
			continue
		}
		if extclient.Mutex != nil {
			extclient.Mutex.Lock()
		}
		if _, ok := extclient.Tags[tag.ID]; ok {
			nMap[extclient.ClientID] = extclient.ConvertToStaticNode()
		}
		if extclient.Mutex != nil {
			extclient.Mutex.Unlock()
		}
	}
	return nMap
}

func GetStaticNodeWithTag(tagID models.TagID) map[string]models.Node {
	nMap := make(map[string]models.Node)
	tag, err := GetTag(tagID)
	if err != nil {
		return nMap
	}
	extclients, err := GetNetworkExtClients(tag.Network.String())
	if err != nil {
		return nMap
	}
	for _, extclient := range extclients {
		nMap[extclient.ClientID] = extclient.ConvertToStaticNode()
	}
	return nMap
}
