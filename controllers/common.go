package controller

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/crypto/bcrypt"
)

func GetPeersList(networkName string) ([]models.PeersResponse, error) {

	var peers []models.PeersResponse
	collection, err := database.FetchRecords(database.NODES_TABLE_NAME)

	if err != nil {
		return peers, err
	}

	for _, value := range collection {
		var node models.Node
		var peer models.PeersResponse
		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			continue
		}
		err = json.Unmarshal([]byte(value), &peer)
		if err != nil {
			continue
		}
		if node.Network == networkName && !node.IsPending {
			peers = append(peers, peer)
		}
	}

	return peers, err
}

func GetExtPeersList(networkName string, macaddress string) ([]models.ExtPeersResponse, error) {

	var peers []models.ExtPeersResponse
	records, err := database.FetchRecords(database.EXT_CLIENT_TABLE_NAME)

	if err != nil {
		return peers, err
	}

	for _, value := range records {
		var peer models.ExtPeersResponse
		var extClient models.ExtClient
		err = json.Unmarshal([]byte(value), &peer)
		err = json.Unmarshal([]byte(value), &extClient)
		if err != nil {
			functions.PrintUserLog("netmaker", "failed to unmarshal ext client", 2)
			continue
		}
		if extClient.Network == networkName && extClient.IngressGatewayID == macaddress {
			peers = append(peers, peer)
		}
	}
	return peers, err
}

func ValidateNodeCreate(networkName string, node models.Node) error {
	v := validator.New()
	_ = v.RegisterValidation("macaddress_unique", func(fl validator.FieldLevel) bool {
		isFieldUnique, _ := functions.IsMacAddressUnique(node.MacAddress, networkName)
		return isFieldUnique
	})
	_ = v.RegisterValidation("network_exists", func(fl validator.FieldLevel) bool {
		_, err := node.GetNetwork()
		return err == nil
	})
	_ = v.RegisterValidation("in_charset", func(fl validator.FieldLevel) bool {
		isgood := functions.NameInNodeCharSet(node.Name)
		return isgood
	})
	err := v.Struct(node)

	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			fmt.Println(e)
		}
	}
	return err
}

func ValidateNodeUpdate(networkName string, node models.NodeUpdate) error {
	v := validator.New()
	_ = v.RegisterValidation("network_exists", func(fl validator.FieldLevel) bool {
		_, err := functions.GetParentNetwork(networkName)
		return err == nil
	})
	_ = v.RegisterValidation("in_charset", func(fl validator.FieldLevel) bool {
		isgood := functions.NameInNodeCharSet(node.Name)
		return isgood
	})
	err := v.Struct(node)
	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			fmt.Println(e)
		}
	}
	return err
}

func UpdateNode(nodechange models.NodeUpdate, node models.Node) (models.Node, error) {
	//Question: Is there a better way  of doing  this than a bunch of "if" statements? probably...
	//Eventually, lets have a better way to check if any of the fields are filled out...
	oldkey, err := functions.GetRecordKey(node.MacAddress, node.Network)
	if err != nil {
		return node, err
	}

	notifynetwork := false

	if nodechange.Address != "" {
		node.Address = nodechange.Address
		notifynetwork = true
	}
	if nodechange.Address6 != "" {
		node.Address6 = nodechange.Address6
		notifynetwork = true
	}
	if nodechange.Name != "" {
		node.Name = nodechange.Name
	}
	if nodechange.LocalAddress != "" {
		node.LocalAddress = nodechange.LocalAddress
	}
	if nodechange.ListenPort != 0 {
		node.ListenPort = nodechange.ListenPort
	}
	if nodechange.ExpirationDateTime != 0 {
		node.ExpirationDateTime = nodechange.ExpirationDateTime
	}
	if nodechange.PostDown != "" {
		node.PostDown = nodechange.PostDown
	}
	if nodechange.Interface != "" {
		node.Interface = nodechange.Interface
	}
	if nodechange.PostUp != "" {
		node.PostUp = nodechange.PostUp
	}
	if nodechange.AccessKey != "" {
		node.AccessKey = nodechange.AccessKey
	}
	if nodechange.Endpoint != "" {
		node.Endpoint = nodechange.Endpoint
		notifynetwork = true
	}
	if nodechange.SaveConfig != nil {
		node.SaveConfig = nodechange.SaveConfig
	}
	if nodechange.PersistentKeepalive != 0 {
		node.PersistentKeepalive = nodechange.PersistentKeepalive
	}
	if nodechange.Password != "" {
		err := bcrypt.CompareHashAndPassword([]byte(nodechange.Password), []byte(node.Password))
		if err != nil && nodechange.Password != node.Password {
			hash, err := bcrypt.GenerateFromPassword([]byte(nodechange.Password), 5)
			if err != nil {
				return node, err
			}
			nodechange.Password = string(hash)
			node.Password = nodechange.Password
		}
	}
	if nodechange.MacAddress != "" {
		node.MacAddress = nodechange.MacAddress
	}
	if nodechange.PublicKey != "" {
		node.PublicKey = nodechange.PublicKey
		node.KeyUpdateTimeStamp = time.Now().Unix()
		notifynetwork = true
	}
	newkey, err := functions.GetRecordKey(node.MacAddress, node.Network)
	if err != nil {
		return node, err
	}
	if oldkey != newkey {
		if err := database.DeleteRecord(database.NODES_TABLE_NAME, oldkey); err != nil {
			return models.Node{}, err
		}
	}
	value, err := json.Marshal(&node)
	if err != nil {
		return models.Node{}, err
	}

	err = database.Insert(newkey, string(value), database.NODES_TABLE_NAME)

	if notifynetwork {
		err = SetNetworkNodesLastModified(node.Network)
	}
	if servercfg.IsDNSMode() {
		err = SetDNS()
	}

	return node, err
}

func DeleteNode(macaddress string, network string) error {

	key, err := functions.GetRecordKey(macaddress, network)
	if err != nil {
		return err
	}
	if err = database.DeleteRecord(database.NODES_TABLE_NAME, key); err != nil {
		return err
	}

	err = SetNetworkNodesLastModified(network)
	if servercfg.IsDNSMode() {
		err = SetDNS()
	}

	return err
}

func DeleteIntClient(clientid string) (bool, error) {

	err := database.DeleteRecord(database.INT_CLIENTS_TABLE_NAME, clientid)
	if err != nil {
		return false, err
	}

	return true, nil
}

func GetNode(macaddress string, network string) (models.Node, error) {

	var node models.Node

	key, err := functions.GetRecordKey(macaddress, network)
	if err != nil {
		return node, err
	}
	data, err := database.FetchRecord(database.NODES_TABLE_NAME, key)
	if err != nil {
		return node, err
	}
	if err = json.Unmarshal([]byte(data), &node); err != nil {
		return node, err
	}

	return node, err
}

func GetIntClient(clientid string) (models.IntClient, error) {

	var client models.IntClient

	value, err := database.FetchRecord(database.INT_CLIENTS_TABLE_NAME, clientid)
	if err != nil {
		return client, err
	}
	if err = json.Unmarshal([]byte(value), &client); err != nil {
		return models.IntClient{}, err
	}
	return client, nil
}

func CreateNode(node models.Node, networkName string) (models.Node, error) {

	//encrypt that password so we never see it again
	hash, err := bcrypt.GenerateFromPassword([]byte(node.Password), 5)

	if err != nil {
		return node, err
	}
	//set password to encrypted password
	node.Password = string(hash)

	node.Network = networkName

	//node.SetDefaults()
	//Umm, why am I doing this again?
	//TODO: Why am I using a local function instead of the struct function? I really dont know.
	//I think I thought it didn't work but uhhh...idk
	node.SetDefaults()

	//Another DB call here...Inefficient
	//Anyways, this scrolls through all the IP Addresses in the network range and checks against nodes
	//until one is open and then returns it
	node.Address, err = functions.UniqueAddress(networkName)
	if err != nil {
		return node, err
	}

	node.Address6, err = functions.UniqueAddress6(networkName)

	if err != nil {
		return node, err
	}

	//IDK why these aren't a part of "set defaults. Pretty dumb.
	//TODO: This is dumb. Consolidate and fix.
	node.SetLastModified()
	node.SetDefaultName()
	node.SetLastCheckIn()
	node.SetLastPeerUpdate()
	node.KeyUpdateTimeStamp = time.Now().Unix()

	//Create a JWT for the node
	tokenString, _ := functions.CreateJWT(node.MacAddress, networkName)

	if tokenString == "" {
		//returnErrorResponse(w, r, errorResponse)
		return node, err
	}
	key, err := functions.GetRecordKey(node.MacAddress, node.Network)
	if err != nil {
		return node, err
	}
	nodebytes, err := json.Marshal(&node)
	if err != nil {
		return node, err
	}

	err = database.Insert(key, string(nodebytes), database.NODES_TABLE_NAME)
	if err != nil {
		return node, err
	}
	//return response for if node  is pending
	if !node.IsPending {

		functions.DecrimentKey(node.Network, node.AccessKey)

	}

	SetNetworkNodesLastModified(node.Network)
	if servercfg.IsDNSMode() {
		err = SetDNS()
	}
	return node, err
}

func NodeCheckIn(node models.Node, networkName string) (models.CheckInResponse, error) {

	var response models.CheckInResponse

	parentnetwork, err := functions.GetParentNetwork(networkName)
	if err != nil {
		err = fmt.Errorf("%w; Couldnt retrieve Network "+networkName+": ", err)
		return response, err
	}

	parentnode, err := GetNode(node.MacAddress, networkName)
	if err != nil {
		err = fmt.Errorf("%w; Couldnt Get Node "+node.MacAddress, err)
		return response, err
	}
	if parentnode.IsPending {
		err = fmt.Errorf("%w; Node checking in is still pending: "+node.MacAddress, err)
		response.IsPending = true
		return response, err
	}

	networklm := parentnetwork.NetworkLastModified
	peerslm := parentnetwork.NodesLastModified
	gkeyupdate := parentnetwork.KeyUpdateTimeStamp
	nkeyupdate := parentnode.KeyUpdateTimeStamp
	peerlistlm := parentnode.LastPeerUpdate
	parentnodelm := parentnode.LastModified
	parentnodelastcheckin := parentnode.LastCheckIn

	if parentnodelastcheckin < parentnodelm {
		response.NeedConfigUpdate = true
	}

	if parentnodelm < networklm {
		response.NeedConfigUpdate = true
	}
	if peerlistlm < peerslm {
		response.NeedPeerUpdate = true
	}
	if nkeyupdate < gkeyupdate {
		response.NeedKeyUpdate = true
	}
	if time.Now().Unix() > parentnode.ExpirationDateTime {
		response.NeedDelete = true
		err = DeleteNode(node.MacAddress, networkName)
	} else {
		err = TimestampNode(parentnode, true, false, false)

		if err != nil {
			err = fmt.Errorf("%w; Couldnt Timestamp Node: ", err)
			return response, err
		}
	}
	response.Success = true

	return response, err
}

func SetNetworkNodesLastModified(networkName string) error {

	timestamp := time.Now().Unix()

	network, err := functions.GetParentNetwork(networkName)
	if err != nil {
		return err
	}
	network.NodesLastModified = timestamp
	data, err := json.Marshal(&network)
	if err != nil {
		return err
	}
	err = database.Insert(networkName, string(data), database.NETWORKS_TABLE_NAME)
	if err != nil {
		return err
	}
	return nil
}

func TimestampNode(node models.Node, updatecheckin bool, updatepeers bool, updatelm bool) error {
	if updatelm {
		node.SetLastModified()
	}
	if updatecheckin {
		node.SetLastCheckIn()
	}
	if updatepeers {
		node.SetLastPeerUpdate()
	}

	key, err := functions.GetRecordKey(node.MacAddress, node.Network)
	if err != nil {
		return err
	}
	value, err := json.Marshal(&node)
	if err != nil {
		return err
	}

	err = database.Insert(key, string(value), database.NODES_TABLE_NAME)

	return err
}
