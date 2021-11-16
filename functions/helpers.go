package functions

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

// PrintUserLog - prints a log with a given username
func PrintUserLog(username string, message string, loglevel int) {
	log.SetFlags(log.Flags() &^ (log.Llongfile | log.Lshortfile))
	if int32(loglevel) <= servercfg.GetVerbose() && servercfg.GetVerbose() != 0 {
		log.Println("[netmaker]", username, message)
	}
}

// ParseNetwork - parses a network into a model
func ParseNetwork(value string) (models.Network, error) {
	var network models.Network
	err := json.Unmarshal([]byte(value), &network)
	return network, err
}

// ParseNode - parses a node into a model
func ParseNode(value string) (models.Node, error) {
	var node models.Node
	err := json.Unmarshal([]byte(value), &node)
	return node, err
}

// ParseExtClient - parses an extclient into a model
func ParseExtClient(value string) (models.ExtClient, error) {
	var extClient models.ExtClient
	err := json.Unmarshal([]byte(value), &extClient)
	return extClient, err
}

// ParseIntClient - parses int client
func ParseIntClient(value string) (models.IntClient, error) {
	var intClient models.IntClient
	err := json.Unmarshal([]byte(value), &intClient)
	return intClient, err
}

//Takes in an arbitrary field and value for field and checks to see if any other
//node has that value for the same field within the network

// SliceContains - sees if a slice contains something
func SliceContains(slice []string, item string) bool {
	set := make(map[string]struct{}, len(slice))
	for _, s := range slice {
		set[s] = struct{}{}
	}

	_, ok := set[item]
	return ok
}

// GetPeersList - gets peers for given network
func GetPeersList(networkName string) ([]models.PeersResponse, error) {

	var peers []models.PeersResponse
	collection, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		return peers, err
	}

	for _, value := range collection {

		var peer models.PeersResponse
		err := json.Unmarshal([]byte(value), &peer)
		if err != nil {
			continue // try the rest
		}
		peers = append(peers, peer)
	}

	return peers, err
}

// GetIntPeersList - get int peers list
func GetIntPeersList() ([]models.PeersResponse, error) {

	var peers []models.PeersResponse
	records, err := database.FetchRecords(database.INT_CLIENTS_TABLE_NAME)

	if err != nil {
		return peers, err
	}
	// parse the peers

	for _, value := range records {

		var peer models.PeersResponse
		err := json.Unmarshal([]byte(value), &peer)
		if err != nil {
			log.Fatal(err)
		}
		// add the node to our node array
		//maybe better to just return this? But then that's just GetNodes...
		peers = append(peers, peer)
	}

	return peers, err
}

// GetServerIntClient - get server int client
func GetServerIntClient() (*models.IntClient, error) {

	intClients, err := database.FetchRecords(database.INT_CLIENTS_TABLE_NAME)
	for _, value := range intClients {
		var intClient models.IntClient
		err = json.Unmarshal([]byte(value), &intClient)
		if err != nil {
			return nil, err
		}
		if intClient.IsServer == "yes" && intClient.Network == "comms" {
			return &intClient, nil
		}
	}
	return nil, err
}

// NetworkExists - check if network exists
func NetworkExists(name string) (bool, error) {

	var network string
	var err error
	if network, err = database.FetchRecord(database.NETWORKS_TABLE_NAME, name); err != nil {
		return false, err
	}
	return len(network) > 0, nil
}

// NetworkNodesUpdateAction - updates action of network nodes
func NetworkNodesUpdateAction(networkName string, action string) error {

	collections, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		if database.IsEmptyRecord(err) {
			return nil
		}
		return err
	}

	for _, value := range collections {
		var node models.Node
		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			fmt.Println("error in node address assignment!")
			return err
		}
		if action == models.NODE_UPDATE_KEY && node.IsStatic == "yes" {
			continue
		}
		if node.Network == networkName {
			node.Action = action
			data, err := json.Marshal(&node)
			if err != nil {
				return err
			}
			node.SetID()
			database.Insert(node.ID, string(data), database.NODES_TABLE_NAME)
		}
	}
	return nil
}

// NetworkNodesUpdatePullChanges - tells nodes on network to pull
func NetworkNodesUpdatePullChanges(networkName string) error {

	collections, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		if database.IsEmptyRecord(err) {
			return nil
		}
		return err
	}

	for _, value := range collections {
		var node models.Node
		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			fmt.Println("error in node address assignment!")
			return err
		}
		if node.Network == networkName {
			node.PullChanges = "yes"
			data, err := json.Marshal(&node)
			if err != nil {
				return err
			}
			node.SetID()
			database.Insert(node.ID, string(data), database.NODES_TABLE_NAME)
		}
	}

	return nil
}

// IsNetworkDisplayNameUnique - checks if network display name unique
func IsNetworkDisplayNameUnique(name string) (bool, error) {

	isunique := true

	dbs, err := logic.GetNetworks()
	if err != nil {
		return database.IsEmptyRecord(err), err
	}

	for i := 0; i < len(dbs); i++ {

		if name == dbs[i].DisplayName {
			isunique = false
		}
	}

	return isunique, nil
}

// IsMacAddressUnique - checks if mac is unique
func IsMacAddressUnique(macaddress string, networkName string) (bool, error) {

	_, err := database.FetchRecord(database.NODES_TABLE_NAME, macaddress+"###"+networkName)
	if err != nil {
		return database.IsEmptyRecord(err), err
	}

	return true, nil
}

// GetNetworkNonServerNodeCount - get number of network non server nodes
func GetNetworkNonServerNodeCount(networkName string) (int, error) {

	collection, err := database.FetchRecords(database.NODES_TABLE_NAME)
	count := 0
	if err != nil && !database.IsEmptyRecord(err) {
		return count, err
	}
	for _, value := range collection {
		var node models.Node
		if err = json.Unmarshal([]byte(value), &node); err != nil {
			return count, err
		} else {
			if node.Network == networkName && node.IsServer != "yes" {
				count++
			}
		}
	}

	return count, nil
}

// IsKeyValidGlobal - checks if a key is valid globally
func IsKeyValidGlobal(keyvalue string) bool {

	networks, _ := logic.GetNetworks()
	var key models.AccessKey
	foundkey := false
	isvalid := false
	for _, network := range networks {
		for i := len(network.AccessKeys) - 1; i >= 0; i-- {
			currentkey := network.AccessKeys[i]
			if currentkey.Value == keyvalue {
				key = currentkey
				foundkey = true
				break
			}
		}
		if foundkey {
			break
		}
	}
	if foundkey {
		if key.Uses > 0 {
			isvalid = true
		}
	}
	return isvalid
}

//TODO: Contains a fatal error return. Need to change
//This just gets a network object from a network name
//Should probably just be GetNetwork. kind of a dumb name.
//Used in contexts where it's not the Parent network.

//Similar to above but checks if Cidr range is valid
//At least this guy's got some print statements
//still not good error handling

//This  checks to  make sure a network name is valid.
//Switch to REGEX?

// NameInNetworkCharSet - see if name is in charset for networks
func NameInNetworkCharSet(name string) bool {

	charset := "abcdefghijklmnopqrstuvwxyz1234567890-_."

	for _, char := range name {
		if !strings.Contains(charset, strings.ToLower(string(char))) {
			return false
		}
	}
	return true
}

// NameInDNSCharSet - name in dns char set
func NameInDNSCharSet(name string) bool {

	charset := "abcdefghijklmnopqrstuvwxyz1234567890-."

	for _, char := range name {
		if !strings.Contains(charset, strings.ToLower(string(char))) {
			return false
		}
	}
	return true
}

// NameInNodeCharSet - name in node char set
func NameInNodeCharSet(name string) bool {

	charset := "abcdefghijklmnopqrstuvwxyz1234567890-"

	for _, char := range name {
		if !strings.Contains(charset, strings.ToLower(string(char))) {
			return false
		}
	}
	return true
}

// RemoveDeletedNode - remove deleted node
func RemoveDeletedNode(nodeid string) bool {
	return database.DeleteRecord(database.DELETED_NODES_TABLE_NAME, nodeid) == nil
}

// DeleteAllIntClients - delete all int clients
func DeleteAllIntClients() error {
	err := database.DeleteAllRecords(database.INT_CLIENTS_TABLE_NAME)
	if err != nil {
		return err
	}
	return nil
}

// GetAllIntClients - get all int clients
func GetAllIntClients() ([]models.IntClient, error) {
	var clients []models.IntClient
	collection, err := database.FetchRecords(database.INT_CLIENTS_TABLE_NAME)

	if err != nil {
		return clients, err
	}

	for _, value := range collection {
		var client models.IntClient
		err := json.Unmarshal([]byte(value), &client)
		if err != nil {
			return []models.IntClient{}, err
		}
		// add node to our array
		clients = append(clients, client)
	}

	return clients, nil
}

// GetAllExtClients - get all ext clients
func GetAllExtClients() ([]models.ExtClient, error) {
	var extclients []models.ExtClient
	collection, err := database.FetchRecords(database.EXT_CLIENT_TABLE_NAME)

	if err != nil {
		return extclients, err
	}

	for _, value := range collection {
		var extclient models.ExtClient
		err := json.Unmarshal([]byte(value), &extclient)
		if err != nil {
			return []models.ExtClient{}, err
		}
		// add node to our array
		extclients = append(extclients, extclient)
	}

	return extclients, nil
}

// GenKey - generates access key
func GenKey() string {

	var seededRand *rand.Rand = rand.New(
		rand.NewSource(time.Now().UnixNano()))

	length := 16
	charset := "abcdefghijklmnopqrstuvwxyz" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

//generate a key value
//we should probably just have 1 random string generator
//that  can be used across all functions
//have a "base string" a "length" and a "charset"

// GenKeyName - generates a key name
func GenKeyName() string {

	var seededRand *rand.Rand = rand.New(
		rand.NewSource(time.Now().UnixNano()))

	length := 5
	charset := "abcdefghijklmnopqrstuvwxyz" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return "key" + string(b)
}

// DeleteKey - deletes a key
func DeleteKey(network models.Network, i int) {

	network.AccessKeys = append(network.AccessKeys[:i],
		network.AccessKeys[i+1:]...)

	if networkData, err := json.Marshal(&network); err != nil {
		return
	} else {
		database.Insert(network.NetID, string(networkData), database.NETWORKS_TABLE_NAME)
	}
}
