//TODO: Consider restructuring  this file/folder    "github.com/gorilla/handlers"

//It may make more sense to split into different files and not call it "helpers"

package functions

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

func PrintUserLog(username string, message string, loglevel int) {
	log.SetFlags(log.Flags() &^ (log.Llongfile | log.Lshortfile))
	if int32(loglevel) <= servercfg.GetVerbose() && servercfg.GetVerbose() != 0 {
		log.Println(username, message)
	}
}

func ParseNetwork(value string) (models.Network, error) {
	var network models.Network
	err := json.Unmarshal([]byte(value), &network)
	return network, err
}

func ParseNode(value string) (models.Node, error) {
	var node models.Node
	err := json.Unmarshal([]byte(value), &node)
	return node, err
}

func ParseExtClient(value string) (models.ExtClient, error) {
	var extClient models.ExtClient
	err := json.Unmarshal([]byte(value), &extClient)
	return extClient, err
}

func ParseIntClient(value string) (models.IntClient, error) {
	var intClient models.IntClient
	err := json.Unmarshal([]byte(value), &intClient)
	return intClient, err
}

//Takes in an arbitrary field and value for field and checks to see if any other
//node has that value for the same field within the network

func GetUser(username string) (models.User, error) {

	var user models.User
	record, err := database.FetchRecord(database.USERS_TABLE_NAME, username)
	if err != nil {
		return user, err
	}
	if err = json.Unmarshal([]byte(record), &user); err != nil {
		return models.User{}, err
	}
	return user, err
}

func SliceContains(slice []string, item string) bool {
	set := make(map[string]struct{}, len(slice))
	for _, s := range slice {
		set[s] = struct{}{}
	}

	_, ok := set[item]
	return ok
}

func CreateServerToken(netID string) (string, error) {
	var network models.Network
	var accesskey models.AccessKey

	network, err := GetParentNetwork(netID)
	if err != nil {
		return "", err
	}

	var accessToken models.AccessToken
	servervals := models.ServerConfig{
		APIConnString:  "127.0.0.1" + servercfg.GetAPIPort(),
		GRPCConnString: "127.0.0.1" + servercfg.GetGRPCPort(),
		GRPCSSL:        "off",
	}
	accessToken.ServerConfig = servervals
	accessToken.ClientConfig.Network = netID
	accessToken.ClientConfig.Key = GenKey()

	accesskey.Name = GenKeyName()
	accesskey.Value = GenKey()
	accesskey.Uses = 1

	tokenjson, err := json.Marshal(accessToken)
	if err != nil {
		return accesskey.AccessString, err
	}

	accesskey.AccessString = base64.StdEncoding.EncodeToString([]byte(tokenjson))

	network.AccessKeys = append(network.AccessKeys, accesskey)
	if data, err := json.Marshal(network); err != nil {
		return "", err
	} else {
		database.Insert(netID, string(data), database.NETWORKS_TABLE_NAME)
	}

	return accesskey.AccessString, nil
}

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

func NetworkExists(name string) (bool, error) {

	var network string
	var err error
	if network, err = database.FetchRecord(database.NETWORKS_TABLE_NAME, name); err != nil {
		return false, err
	}
	return len(network) > 0, nil
}
func GetRecordKey(id string, network string) (string, error) {
	if id == "" || network == "" {
		return "", errors.New("unable to get record key")
	}
	return id + "###" + network, nil
}

//TODO: This is  very inefficient (N-squared). Need to find a better way.
//Takes a list of  nodes in a network and iterates through
//for each node, it gets a unique address. That requires checking against all other nodes once more
func UpdateNetworkNodeAddresses(networkName string) error {

	collections, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		return err
	}

	for _, value := range collections {

		var node models.Node
		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			fmt.Println("error in node address assignment!")
			return err
		}
		ipaddr, iperr := UniqueAddress(networkName)
		if iperr != nil {
			fmt.Println("error in node  address assignment!")
			return iperr
		}

		node.Address = ipaddr
		data, err := json.Marshal(&node)
		if err != nil {
			return err
		}
		database.Insert(node.MacAddress, string(data), database.NODES_TABLE_NAME)
	}

	return nil
}

func UpdateNetworkLocalAddresses(networkName string) error {

	collection, err := database.FetchRecords(database.NODES_TABLE_NAME)

	if err != nil {
		return err
	}

	for _, value := range collection {

		var node models.Node

		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			fmt.Println("error in node address assignment!")
			return err
		}
		ipaddr, iperr := UniqueAddress(networkName)
		if iperr != nil {
			fmt.Println("error in node  address assignment!")
			return iperr
		}

		node.Address = ipaddr
		newNodeData, err := json.Marshal(&node)
		if err != nil {
			fmt.Println("error in node  address assignment!")
			return err
		}
		database.Insert(node.MacAddress, string(newNodeData), database.NODES_TABLE_NAME)
	}

	return nil
}

func IsNetworkDisplayNameUnique(name string) (bool, error) {

	isunique := true

	dbs, err := models.GetNetworks()
	if err != nil {
		return false, err
	}

	for i := 0; i < len(dbs); i++ {

		if name == dbs[i].DisplayName {
			isunique = false
		}
	}

	return isunique, nil
}

func IsMacAddressUnique(macaddress string, networkName string) (bool, error) {

	collection, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		return false, err
	}
	for _, value := range collection {
		var node models.Node
		if err = json.Unmarshal([]byte(value), &node); err != nil {
			return false, err
		} else {
			if node.MacAddress == macaddress && node.Network == networkName {
				return false, nil
			}
		}
	}

	return true, nil
}

func GetNetworkNodeNumber(networkName string) (int, error) {

	collection, err := database.FetchRecords(database.NODES_TABLE_NAME)
	count := 0
	if err != nil {
		return count, err
	}
	for _, value := range collection {
		var node models.Node
		if err = json.Unmarshal([]byte(value), &node); err != nil {
			return count, err
		} else {
			if node.Network == networkName {
				count++
			}
		}
	}

	return count, nil
}

//Checks to see if access key is valid
//Does so by checking against all keys and seeing if any have the same value
//may want to hash values before comparing...consider this
//TODO: No error handling!!!!
func IsKeyValid(networkname string, keyvalue string) bool {

	network, _ := GetParentNetwork(networkname)
	var key models.AccessKey
	foundkey := false
	isvalid := false

	for i := len(network.AccessKeys) - 1; i >= 0; i-- {
		currentkey := network.AccessKeys[i]
		if currentkey.Value == keyvalue {
			key = currentkey
			foundkey = true
		}
	}
	if foundkey {
		if key.Uses > 0 {
			isvalid = true
		}
	}
	return isvalid
}

func IsKeyValidGlobal(keyvalue string) bool {

	networks, _ := models.GetNetworks()
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
func GetParentNetwork(networkname string) (models.Network, error) {

	var network models.Network
	networkData, err := database.FetchRecord(database.NETWORKS_TABLE_NAME, networkname)
	if err != nil {
		return network, err
	}
	if err = json.Unmarshal([]byte(networkData), &network); err != nil {
		return models.Network{}, err
	}
	return network, nil
}

func IsIpNet(host string) bool {
	return net.ParseIP(host) != nil
}

//Similar to above but checks if Cidr range is valid
//At least this guy's got some print statements
//still not good error handling
func IsIpCIDR(host string) bool {

	ip, ipnet, err := net.ParseCIDR(host)

	if err != nil {
		fmt.Println(err)
		fmt.Println("Address Range is not valid!")
		return false
	}

	return ip != nil && ipnet != nil
}

//This is used to validate public keys (make sure they're base64 encoded like all public keys should be).
func IsBase64(s string) bool {
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}

//This  checks to  make sure a network name is valid.
//Switch to REGEX?
func NameInNetworkCharSet(name string) bool {

	charset := "abcdefghijklmnopqrstuvwxyz1234567890-_."

	for _, char := range name {
		if !strings.Contains(charset, strings.ToLower(string(char))) {
			return false
		}
	}
	return true
}

func NameInDNSCharSet(name string) bool {

	charset := "abcdefghijklmnopqrstuvwxyz1234567890-."

	for _, char := range name {
		if !strings.Contains(charset, strings.ToLower(string(char))) {
			return false
		}
	}
	return true
}

func NameInNodeCharSet(name string) bool {

	charset := "abcdefghijklmnopqrstuvwxyz1234567890-"

	for _, char := range name {
		if !strings.Contains(charset, strings.ToLower(string(char))) {
			return false
		}
	}
	return true
}

//This returns a node based on its mac address.
//The mac address acts as the Unique ID for nodes.
//Is this a dumb thing to do? I thought it was cool but maybe it's dumb.
//It doesn't really provide a tangible benefit over a random ID
func GetNodeByMacAddress(network string, macaddress string) (models.Node, error) {

	var node models.Node

	key, err := GetRecordKey(macaddress, network)
	if err != nil {
		return node, err
	}

	record, err := database.FetchRecord(database.NODES_TABLE_NAME, key)
	log.Println("RECORD:",record)
	if err != nil {
		return models.Node{}, err
	}

	if err = json.Unmarshal([]byte(record), &node); err != nil {
		return models.Node{}, err
	}

	return node, nil
}

func DeleteAllIntClients() error {
	err := database.DeleteAllRecords(database.INT_CLIENTS_TABLE_NAME)
	if err != nil {
		return err
	}
	return nil
}

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

//This returns a unique address for a node to use
//it iterates through the list of IP's in the subnet
//and checks against all nodes to see if it's taken, until it finds one.
//TODO: We do not handle a case where we run out of addresses.
//We will need to handle that eventually
func UniqueAddress(networkName string) (string, error) {

	var network models.Network
	network, err := GetParentNetwork(networkName)
	if err != nil {
		fmt.Println("UniqueAddress encountered  an error")
		return "666", err
	}

	offset := true
	ip, ipnet, err := net.ParseCIDR(network.AddressRange)
	if err != nil {
		fmt.Println("UniqueAddress encountered  an error")
		return "666", err
	}
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); Inc(ip) {
		if offset {
			offset = false
			continue
		}
		if networkName == "comms" {
			if IsIPUnique(networkName, ip.String(), database.INT_CLIENTS_TABLE_NAME, false) {
				return ip.String(), err
			}
		} else {
			if IsIPUnique(networkName, ip.String(), database.NODES_TABLE_NAME, false) && IsIPUnique(networkName, ip.String(), database.EXT_CLIENT_TABLE_NAME, false) {
				return ip.String(), err
			}
		}
	}

	//TODO
	err1 := errors.New("ERROR: No unique addresses available. Check network subnet.")
	return "W1R3: NO UNIQUE ADDRESSES AVAILABLE", err1
}

func UniqueAddress6(networkName string) (string, error) {

	var network models.Network
	network, err := GetParentNetwork(networkName)
	if err != nil {
		fmt.Println("Network Not Found")
		return "", err
	}
	if network.IsDualStack == "no" {
		if networkName != "comms" {
			return "", nil
		}
	}

	offset := true
	ip, ipnet, err := net.ParseCIDR(network.AddressRange6)
	if err != nil {
		fmt.Println("UniqueAddress6 encountered  an error")
		return "666", err
	}
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); Inc(ip) {
		if offset {
			offset = false
			continue
		}
		if networkName == "comms" {
			if IsIPUnique(networkName, ip.String(), database.INT_CLIENTS_TABLE_NAME, true) {
				return ip.String(), err
			}
		} else {
			if IsIPUnique(networkName, ip.String(), database.NODES_TABLE_NAME, true) {
				return ip.String(), err
			}
		}
	}
	//TODO
	err1 := errors.New("ERROR: No unique addresses available. Check network subnet.")
	return "W1R3: NO UNIQUE ADDRESSES AVAILABLE", err1
}

//generate an access key value
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

func IsIPUnique(network string, ip string, tableName string, isIpv6 bool) bool {

	isunique := true
	collection, err := database.FetchRecords(tableName)

	if err != nil {
		return isunique
	}

	for _, value := range collection { // filter
		var node models.Node
		if err = json.Unmarshal([]byte(value), &node); err != nil {
			continue
		}
		if isIpv6 {
			if node.Address6 == ip && node.Network == network {
				return false
			}
		} else {
			if node.Address == ip && node.Network == network {
				return false
			}
		}
	}

	return isunique
}

//called once key has been used by createNode
//reduces value by one and deletes if necessary
func DecrimentKey(networkName string, keyvalue string) {

	var network models.Network

	network, err := GetParentNetwork(networkName)
	if err != nil {
		return
	}

	for i := len(network.AccessKeys) - 1; i >= 0; i-- {

		currentkey := network.AccessKeys[i]
		if currentkey.Value == keyvalue {
			network.AccessKeys[i].Uses--
			if network.AccessKeys[i].Uses < 1 {
				network.AccessKeys = append(network.AccessKeys[:i],
					network.AccessKeys[i+1:]...)
				break
			}
		}
	}

	if newNetworkData, err := json.Marshal(&network); err != nil {
		PrintUserLog("netmaker", "failed to decrement key", 2)
		return
	} else {
		database.Insert(network.NetID, string(newNetworkData), database.NETWORKS_TABLE_NAME)
	}
}

//takes the logic from controllers.deleteKey
func DeleteKey(network models.Network, i int) {

	network.AccessKeys = append(network.AccessKeys[:i],
		network.AccessKeys[i+1:]...)

	if networkData, err := json.Marshal(&network); err != nil {
		return
	} else {
		database.Insert(network.NetID, string(networkData), database.NETWORKS_TABLE_NAME)
	}
}

//increments an IP over the previous
func Inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
