//TODO: Consider restructuring  this file/folder    "github.com/gorilla/handlers"

//It may make more sense to split into different files and not call it "helpers"

package functions

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"time"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mongoconn"
	"github.com/gravitl/netmaker/servercfg"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//Takes in an arbitrary field and value for field and checks to see if any other
//node has that value for the same field within the network

func CreateServerToken(netID string) (string, error) {
	var network models.Network
	var accesskey models.AccessKey

	network, err := GetParentNetwork(netID)
	if err != nil {
		return "", err
	}

	accesskey.Name = GenKeyName()
	accesskey.Value = GenKey()
	accesskey.Uses = 1
	address := "127.0.0.1:" + servercfg.GetGRPCPort()

	privAddr := ""
	if *network.IsLocal {
		privAddr = network.LocalRange
	}
	accessstringdec := servercfg.GetGRPCWGPort() + "|" + address + "|" + address + "|" + netID + "|" + accesskey.Value + "|" + privAddr


	log.Println("Access String: " + accessstringdec)
	accesskey.AccessString = base64.StdEncoding.EncodeToString([]byte(accessstringdec))

	network.AccessKeys = append(network.AccessKeys, accesskey)

	collection := mongoconn.Client.Database("netmaker").Collection("networks")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	// Create filter
	filter := bson.M{"netid": netID}

	// Read update model from body request
	fmt.Println("Adding key to " + network.NetID)

	// prepare update model.
	update := bson.D{
		{"$set", bson.D{
			{"accesskeys", network.AccessKeys},
		}},
	}

	errN := collection.FindOneAndUpdate(ctx, filter, update).Decode(&network)

	defer cancel()

	if errN != nil {
		return "", errN
	}
	return accesskey.AccessString, nil
}

func GetPeersList(networkName string) ([]models.PeersResponse, error) {

        var peers []models.PeersResponse

        //Connection mongoDB with mongoconn class
        collection := mongoconn.Client.Database("netmaker").Collection("nodes")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        //Get all nodes in the relevant network which are NOT in pending state
        filter := bson.M{"network": networkName, "ispending": false}
        cur, err := collection.Find(ctx, filter)

        if err != nil {
                return peers, err
        }

        // Close the cursor once finished and cancel if it takes too long
        defer cancel()

        for cur.Next(context.TODO()) {

                var peer models.PeersResponse
                err := cur.Decode(&peer)
                if err != nil {
                        log.Fatal(err)
                }

                // add the node to our node array
                //maybe better to just return this? But then that's just GetNodes...
                peers = append(peers, peer)
        }

        //Uh oh, fatal error! This needs some better error handling
        //TODO: needs appropriate error handling so the server doesnt shut down.
        if err := cur.Err(); err != nil {
                log.Fatal(err)
        }

        return peers, err
}

func GetIntPeersList() ([]models.PeersResponse, error) {

        var peers []models.PeersResponse

        collection := mongoconn.Client.Database("netmaker").Collection("intclients")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"isserver": ""}

        cur, err := collection.Find(ctx, filter)

        if err != nil {
                return peers, err
        }

        // Close the cursor once finished and cancel if it takes too long
        defer cancel()

        for cur.Next(context.TODO()) {

                var peer models.PeersResponse
                err := cur.Decode(&peer)
                if err != nil {
                        log.Fatal(err)
                }

                // add the node to our node array
                //maybe better to just return this? But then that's just GetNodes...
                peers = append(peers, peer)
        }

        //Uh oh, fatal error! This needs some better error handling
        //TODO: needs appropriate error handling so the server doesnt shut down.
        if err := cur.Err(); err != nil {
                log.Fatal(err)
        }

        return peers, err
}

func IsFieldUnique(network string, field string, value string) bool {

	var node models.Node
	isunique := true

	collection := mongoconn.Client.Database("netmaker").Collection("nodes")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	filter := bson.M{field: value, "network": network}

	err := collection.FindOne(ctx, filter).Decode(&node)

	defer cancel()

	if err != nil {
		return isunique
	}

	if node.Name != "" {
		isunique = false
	}

	return isunique
}

func ServerIntClientExists() (bool, error) {

        collection := mongoconn.Client.Database("netmaker").Collection("intclients")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"isserver": "yes"}

        var result bson.M
        err := collection.FindOne(ctx, filter).Decode(&result)

        defer cancel()

        if err != nil {
                if err == mongo.ErrNoDocuments {
                        return false, nil
                }
        }
        return true, err
}

func NetworkExists(name string) (bool, error) {

	collection := mongoconn.Client.Database("netmaker").Collection("networks")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	filter := bson.M{"netid": name}

	var result bson.M
	err := collection.FindOne(ctx, filter).Decode(&result)

	defer cancel()

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
	}
	return true, err
}

//TODO: This is  very inefficient (N-squared). Need to find a better way.
//Takes a list of  nodes in a network and iterates through
//for each node, it gets a unique address. That requires checking against all other nodes once more
func UpdateNetworkNodeAddresses(networkName string) error {

	//Connection mongoDB with mongoconn class
	collection := mongoconn.Client.Database("netmaker").Collection("nodes")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	filter := bson.M{"network": networkName}
	cur, err := collection.Find(ctx, filter)

	if err != nil {
		return err
	}

	defer cancel()

	for cur.Next(context.TODO()) {

		var node models.Node

		err := cur.Decode(&node)
		if err != nil {
			fmt.Println("error in node address assignment!")
			return err
		}
		ipaddr, iperr := UniqueAddress(networkName)
		if iperr != nil {
			fmt.Println("error in node  address assignment!")
			return iperr
		}

		filter := bson.M{"macaddress": node.MacAddress}
		update := bson.D{{"$set", bson.D{{"address", ipaddr}}}}

		errN := collection.FindOneAndUpdate(ctx, filter, update).Decode(&node)

		defer cancel()
		if errN != nil {
			return errN
		}
	}

	return err
}

//TODO TODO TODO!!!!!
func UpdateNetworkPrivateAddresses(networkName string) error {

	//Connection mongoDB with mongoconn class
	collection := mongoconn.Client.Database("netmaker").Collection("nodes")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	filter := bson.M{"network": networkName}
	cur, err := collection.Find(ctx, filter)

	if err != nil {
		return err
	}

	defer cancel()

	for cur.Next(context.TODO()) {

		var node models.Node

		err := cur.Decode(&node)
		if err != nil {
			fmt.Println("error in node address assignment!")
			return err
		}
		ipaddr, iperr := UniqueAddress(networkName)
		if iperr != nil {
			fmt.Println("error in node  address assignment!")
			return iperr
		}

		filter := bson.M{"macaddress": node.MacAddress}
		update := bson.D{{"$set", bson.D{{"address", ipaddr}}}}

		errN := collection.FindOneAndUpdate(ctx, filter, update).Decode(&node)

		defer cancel()
		if errN != nil {
			return errN
		}
	}

	return err
}

//Checks to see if any other networks have the same name (id)
func IsNetworkNameUnique(name string) (bool, error) {

	isunique := true

	dbs, err := ListNetworks()

	if err != nil {
		return false, err
	}

	for i := 0; i < len(dbs); i++ {

		if name == dbs[i].NetID {
			isunique = false
		}
	}

	return isunique, nil
}

func IsNetworkDisplayNameUnique(name string) (bool, error) {

	isunique := true

	dbs, err := ListNetworks()
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

func GetNetworkNodeNumber(networkName string) (int, error) {

	collection := mongoconn.Client.Database("netmaker").Collection("nodes")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	filter := bson.M{"network": networkName}
	count, err := collection.CountDocuments(ctx, filter)
	returncount := int(count)

	//not sure if this is the right way of handling this error...
	if err != nil {
		return 9999, err
	}

	defer cancel()

	return returncount, err
}

//Kind  of a weird name. Should just be GetNetworks I think. Consider changing.
//Anyway, returns all the networks
func ListNetworks() ([]models.Network, error) {

	var networks []models.Network

	collection := mongoconn.Client.Database("netmaker").Collection("networks")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	cur, err := collection.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"_id": 0}))

	if err != nil {
		return networks, err
	}

	defer cancel()

	for cur.Next(context.TODO()) {

		var network models.Network
		err := cur.Decode(&network)
		if err != nil {
			return networks, err
		}

		// add network our array
		networks = append(networks, network)
	}

	if err := cur.Err(); err != nil {
		return networks, err
	}

	return networks, err
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

        networks, _ := ListNetworks()
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
		if foundkey { break }
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

	collection := mongoconn.Client.Database("netmaker").Collection("networks")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	filter := bson.M{"netid": networkname}
	err := collection.FindOne(ctx, filter).Decode(&network)

	defer cancel()

	if err != nil {
		return network, err
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

//This should probably just be called GetNode
//It returns a node based on the ID of the node.
//Why do we need this?
//TODO: Check references. This seems unnecessary.
func GetNodeObj(id primitive.ObjectID) models.Node {

	var node models.Node

	collection := mongoconn.Client.Database("netmaker").Collection("nodes")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	filter := bson.M{"_id": id}
	err := collection.FindOne(ctx, filter).Decode(&node)

	defer cancel()

	if err != nil {
		fmt.Println(err)
		fmt.Println("Did not get the node...")
		return node
	}
	fmt.Println("Got node " + node.Name)
	return node
}

//This  checks to  make sure a network name is valid.
//Switch to REGEX?
func NameInNetworkCharSet(name string) bool {

	charset := "abcdefghijklmnopqrstuvwxyz1234567890-_"

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

	filter := bson.M{"macaddress": macaddress, "network": network}

	collection := mongoconn.Client.Database("netmaker").Collection("nodes")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	err := collection.FindOne(ctx, filter).Decode(&node)

	defer cancel()

	if err != nil {
		return node, err
	}
	return node, nil
}

func DeleteAllIntClients() error {
        collection := mongoconn.Client.Database("netmaker").Collection("intclients")
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        // Filter out them ID's again
        err := collection.Drop(ctx)
        if err != nil {
                return err
        }
        defer cancel()
        return nil
}

func GetAllIntClients() ([]models.IntClient, error) {
        var client models.IntClient
        var clients []models.IntClient
        collection := mongoconn.Client.Database("netmaker").Collection("intclients")
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        // Filter out them ID's again
        cur, err := collection.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"_id": 0}))
        if err != nil {
                return []models.IntClient{}, err
        }
        defer cancel()
        for cur.Next(context.TODO()) {
                err := cur.Decode(&client)
                if err != nil {
                        return []models.IntClient{}, err
                }
                // add node to our array
                clients = append(clients, client)
        }

        //TODO: Fatal error
        if err := cur.Err(); err != nil {
                return []models.IntClient{}, err
        }
        return clients, nil
}

func GetAllExtClients() ([]models.ExtClient, error) {
        var extclient models.ExtClient
        var extclients []models.ExtClient
        collection := mongoconn.Client.Database("netmaker").Collection("extclients")
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        // Filter out them ID's again
        cur, err := collection.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"_id": 0}))
        if err != nil {
                return []models.ExtClient{}, err
        }
        defer cancel()
        for cur.Next(context.TODO()) {
                err := cur.Decode(&extclient)
                if err != nil {
                        return []models.ExtClient{}, err
                }
                // add node to our array
                extclients = append(extclients, extclient)
        }

        //TODO: Fatal error
        if err := cur.Err(); err != nil {
                return []models.ExtClient{}, err
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
                        if IsIPUniqueClients(networkName, ip.String()) {
                                return ip.String(), err
                        }
                } else {
                        if IsIPUnique(networkName, ip.String()) && IsIPUniqueExtClients(networkName, ip.String()) {
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
	if network.IsDualStack == nil || *network.IsDualStack == false {
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
	                if IsIP6UniqueClients(networkName, ip.String()) {
	                        return ip.String(), err
	                }
		} else {
			if IsIP6Unique(networkName, ip.String()) {
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

func IsIPUniqueExtClients(network string, ip string) bool {

        var extclient models.ExtClient

        isunique := true

        collection := mongoconn.Client.Database("netmaker").Collection("extclients")
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"address": ip, "network": network}

        err := collection.FindOne(ctx, filter).Decode(&extclient)

        defer cancel()

        if err != nil {
                return isunique
        }

        if extclient.Address == ip {
                isunique = false
        }
        return isunique
}


//checks if IP is unique in the address range
//used by UniqueAddress
func IsIPUnique(network string, ip string) bool {

	var node models.Node

	isunique := true

	collection := mongoconn.Client.Database("netmaker").Collection("nodes")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	filter := bson.M{"address": ip, "network": network}

	err := collection.FindOne(ctx, filter).Decode(&node)

	defer cancel()

	if err != nil {
		return isunique
	}

	if node.Address == ip {
		isunique = false
	}
	return isunique
}

//checks if IP is unique in the address range
//used by UniqueAddress
func IsIP6Unique(network string, ip string) bool {

	var node models.Node

	isunique := true

	collection := mongoconn.Client.Database("netmaker").Collection("nodes")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	filter := bson.M{"address6": ip, "network": network}

	err := collection.FindOne(ctx, filter).Decode(&node)

	defer cancel()

	if err != nil {
		return isunique
	}

	if node.Address6 == ip {
		isunique = false
	}
	return isunique
}

//checks if IP is unique in the address range
//used by UniqueAddress
func IsIP6UniqueClients(network string, ip string) bool {

        var client models.IntClient

        isunique := true

        collection := mongoconn.Client.Database("netmaker").Collection("intclients")
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"address6": ip, "network": network}

        err := collection.FindOne(ctx, filter).Decode(&client)

        defer cancel()

        if err != nil {
                return isunique
        }

        if client.Address6 == ip {
                isunique = false
        }
        return isunique
}

//checks if IP is unique in the address range
//used by UniqueAddress
func IsIPUniqueClients(network string, ip string) bool {

        var client models.IntClient

        isunique := true

        collection := mongoconn.Client.Database("netmaker").Collection("intclients")
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"address": ip, "network": network}

        err := collection.FindOne(ctx, filter).Decode(&client)

        defer cancel()

        if err != nil {
                return isunique
        }

        if client.Address == ip {
                isunique = false
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
				//this is the part where it will call the delete
				//not sure if there's edge cases I'm missing
				DeleteKey(network, i)
				return
			}
		}
	}

	collection := mongoconn.Client.Database("netmaker").Collection("networks")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	filter := bson.M{"netid": network.NetID}

	update := bson.D{
		{"$set", bson.D{
			{"accesskeys", network.AccessKeys},
		}},
	}
	errN := collection.FindOneAndUpdate(ctx, filter, update).Decode(&network)

	defer cancel()

	if errN != nil {
		return
	}
}

//takes the logic from controllers.deleteKey
func DeleteKey(network models.Network, i int) {

	network.AccessKeys = append(network.AccessKeys[:i],
		network.AccessKeys[i+1:]...)

	collection := mongoconn.Client.Database("netmaker").Collection("networks")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	// Create filter
	filter := bson.M{"netid": network.NetID}

	// prepare update model.
	update := bson.D{
		{"$set", bson.D{
			{"accesskeys", network.AccessKeys},
		}},
	}

	errN := collection.FindOneAndUpdate(ctx, filter, update).Decode(&network)

	defer cancel()

	if errN != nil {
		return
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

func GetAllNodes() ([]models.Node, error) {
	var node models.Node
	var nodes []models.Node
	collection := mongoconn.Client.Database("netmaker").Collection("nodes")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// Filter out them ID's again
	cur, err := collection.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"_id": 0}))
	if err != nil {
		return []models.Node{}, err
	}
	defer cancel()
	for cur.Next(context.TODO()) {
		err := cur.Decode(&node)
		if err != nil {
			return []models.Node{}, err
		}
		// add node to our array
		nodes = append(nodes, node)
	}

	//TODO: Fatal error
	if err := cur.Err(); err != nil {
		return []models.Node{}, err
	}
	return nodes, nil
}

