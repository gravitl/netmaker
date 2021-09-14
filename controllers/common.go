package controller

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/serverctl"
	"golang.org/x/crypto/bcrypt"
)

func GetPeersList(networkName string) ([]models.Node, error) {

	var peers []models.Node
	collection, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		if database.IsEmptyRecord(err) {
			return peers, nil
		}
		functions.PrintUserLog("", err.Error(), 2)
		return nil, err
	}
	udppeers, errN := database.GetPeers(networkName)
	if errN != nil {
		functions.PrintUserLog("", errN.Error(), 2)
	}
	for _, value := range collection {
		var node models.Node
		var peer models.Node
		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			functions.PrintUserLog("", err.Error(), 2)
			continue
		}
		if node.IsEgressGateway == "yes" { // handle egress stuff
			peer.EgressGatewayRanges = node.EgressGatewayRanges
			peer.IsEgressGateway = node.IsEgressGateway
		}
		if node.Network == networkName && node.IsPending != "yes" && node.DoNotPropagate != "yes" {
			if node.IsRelay == "yes" { // handle relay stuff
				peer.RelayAddrs = node.RelayAddrs
				peer.IsRelay = node.IsRelay
			}
			peer.DoNotPropagate = node.DoNotPropagate
			peer.PublicKey = node.PublicKey
			peer.Endpoint = node.Endpoint
			peer.LocalAddress = node.LocalAddress
			peer.ListenPort = node.ListenPort
			peer.AllowedIPs = node.AllowedIPs
			peer.Address = node.Address
			peer.Address6 = node.Address6
			if node.UDPHolePunch == "yes" && errN == nil && functions.CheckEndpoint(udppeers[node.PublicKey]) {
				endpointstring := udppeers[node.PublicKey]
				endpointarr := strings.Split(endpointstring, ":")
				if len(endpointarr) == 2 {
					port, err := strconv.Atoi(endpointarr[1])
					if err == nil {
						peer.Endpoint = endpointarr[0]
						peer.ListenPort = int32(port)
					}
				}
			}
			if node.IsRelay == "yes" {
				peer.AllowedIPs = append(peer.AllowedIPs,node.RelayAddrs...)
			}
			peers = append(peers, peer)
		}
	}

	return peers, err
}

func GetExtPeersList(macaddress string, networkName string) ([]models.ExtPeersResponse, error) {

	var peers []models.ExtPeersResponse
	records, err := database.FetchRecords(database.EXT_CLIENT_TABLE_NAME)

	if err != nil {
		return peers, err
	}

	for _, value := range records {
		var peer models.ExtPeersResponse
		var extClient models.ExtClient
		err = json.Unmarshal([]byte(value), &peer)
		if err != nil {
			functions.PrintUserLog(models.NODE_SERVER_NAME, "failed to unmarshal peer", 2)
			continue
		}
		err = json.Unmarshal([]byte(value), &extClient)
		if err != nil {
			functions.PrintUserLog(models.NODE_SERVER_NAME, "failed to unmarshal ext client", 2)
			continue
		}
		if extClient.Network == networkName && extClient.IngressGatewayID == macaddress {
			peers = append(peers, peer)
		}
	}
	return peers, err
}

/**
 * If being deleted by server, create a record in the DELETED_NODES_TABLE for the client to find
 * If being deleted by the client, delete completely
 */
func DeleteNode(key string, exterminate bool) error {
	var err error
	if !exterminate {
		args := strings.Split(key, "###")
		node, err := GetNode(args[0], args[1])
		if err != nil {
			return err
		}
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
			functions.PrintUserLog("", err.Error(), 2)
		}
	}
	if err := database.DeleteRecord(database.NODES_TABLE_NAME, key); err != nil {
		return err
	}
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
		if data == "" {
			data, err = database.FetchRecord(database.DELETED_NODES_TABLE_NAME, key)
			err = json.Unmarshal([]byte(data), &node)
		}
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

	//encrypt that password so we never see it
	hash, err := bcrypt.GenerateFromPassword([]byte(node.Password), 5)

	if err != nil {
		return node, err
	}
	//set password to encrypted password
	node.Password = string(hash)

	node.Network = networkName
	if node.Name == models.NODE_SERVER_NAME {
		if node.CheckIsServer() {
			node.IsServer = "yes"
		}
	}

	node.SetDefaults()
	node.Address, err = functions.UniqueAddress(networkName)
	if err != nil {
		return node, err
	}
	node.Address6, err = functions.UniqueAddress6(networkName)
	if err != nil {
		return node, err
	}
	//Create a JWT for the node
	tokenString, _ := functions.CreateJWT(node.MacAddress, networkName)
	if tokenString == "" {
		//returnErrorResponse(w, r, errorResponse)
		return node, err
	}
	if servercfg.IsDNSMode() {
		node.DNSOn = "yes"
	}
	err = node.Validate(false)
	if err != nil {
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
	if node.IsPending != "yes" {
		functions.DecrimentKey(node.Network, node.AccessKey)
	}
	SetNetworkNodesLastModified(node.Network)
	if servercfg.IsDNSMode() {
		err = SetDNS()
	}
	return node, err
}

func SetNetworkServerPeers(networkName string) {
	if currentPeersList, err := serverctl.GetPeers(networkName); err == nil {
		if database.SetPeers(currentPeersList, networkName) {
			functions.PrintUserLog(models.NODE_SERVER_NAME, "set new peers on network "+networkName, 1)
		}
	} else {
		functions.PrintUserLog(models.NODE_SERVER_NAME, "could not set peers on network "+networkName, 1)
		functions.PrintUserLog(models.NODE_SERVER_NAME, err.Error(), 1)
	}
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
