package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/models"
)

func createRelay(w http.ResponseWriter, r *http.Request) {
	var relay models.RelayRequest
	var params = mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	err := json.NewDecoder(r.Body).Decode(&relay)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	relay.NetID = params["network"]
	relay.NodeID = params["macaddress"]
	node, err := CreateRelay(relay)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "created relay on node "+relay.NodeID+" on network "+relay.NetID, 1)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

func CreateRelay(relay models.RelayRequest) (models.Node, error) {
	node, err := functions.GetNodeByMacAddress(relay.NetID, relay.NodeID)
	if node.OS == "windows" { // add in darwin later
		return models.Node{}, errors.New(node.OS + " is unsupported for relay")
	}
	if err != nil {
		return models.Node{}, err
	}
	err = ValidateRelay(relay)
	if err != nil {
		return models.Node{}, err
	}
	node.IsRelay = "yes"
	node.RelayAddrs = relay.RelayAddrs

	key, err := functions.GetRecordKey(relay.NodeID, relay.NetID)
	if err != nil {
		return node, err
	}
	node.SetLastModified()
	node.PullChanges = "yes"
	nodeData, err := json.Marshal(&node)
	if err != nil {
		return node, err
	}
	if err = database.Insert(key, string(nodeData), database.NODES_TABLE_NAME); err != nil {
		return models.Node{}, err
	}
	err = SetRelayedNodes("yes", node.Network, node.RelayAddrs)
	if err != nil {
		return node, err
	}

	if err = functions.NetworkNodesUpdatePullChanges(node.Network); err != nil {
		return models.Node{}, err
	}
	return node, nil
}

func SetRelayedNodes(yesOrno string, networkName string, addrs []string) error {

	collections, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		return err
	}

	for _, value := range collections {

		var node models.Node
		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			return err
		}
		if node.Network == networkName {
			for _, addr := range addrs {
				if addr == node.Address || addr == node.Address6 {
					node.IsRelayed = yesOrno
					data, err := json.Marshal(&node)
					if err != nil {
						return err
					}
					node.SetID()
					database.Insert(node.ID, string(data), database.NODES_TABLE_NAME)
				}
			}
		}
	}
	return nil
}

func ValidateRelay(relay models.RelayRequest) error {
	var err error
	//isIp := functions.IsIpCIDR(gateway.RangeString)
	empty := len(relay.RelayAddrs) == 0
	if empty {
		err = errors.New("IP Ranges Cannot Be Empty")
	}
	return err
}

func UpdateRelay(network string, oldAddrs []string, newAddrs []string) {
	time.Sleep(time.Second / 4)
	err := SetRelayedNodes("no", network, oldAddrs)
	if err != nil {
		functions.PrintUserLog("netmaker", err.Error(), 1)
	}
	err = SetRelayedNodes("yes", network, newAddrs)
	if err != nil {
		functions.PrintUserLog("netmaker", err.Error(), 1)
	}
}

func deleteRelay(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	nodeMac := params["macaddress"]
	netid := params["network"]
	node, err := DeleteRelay(netid, nodeMac)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "deleted egress gateway "+nodeMac+" on network "+netid, 1)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

func DeleteRelay(network, macaddress string) (models.Node, error) {

	node, err := functions.GetNodeByMacAddress(network, macaddress)
	if err != nil {
		return models.Node{}, err
	}
	err = SetRelayedNodes("no", node.Network, node.RelayAddrs)
	if err != nil {
		return node, err
	}

	node.IsRelay = "no"
	node.RelayAddrs = []string{}
	node.SetLastModified()
	node.PullChanges = "yes"
	key, err := functions.GetRecordKey(node.MacAddress, node.Network)
	if err != nil {
		return models.Node{}, err
	}
	data, err := json.Marshal(&node)
	if err != nil {
		return models.Node{}, err
	}
	if err = database.Insert(key, string(data), database.NODES_TABLE_NAME); err != nil {
		return models.Node{}, err
	}
	if err = functions.NetworkNodesUpdatePullChanges(network); err != nil {
		return models.Node{}, err
	}
	return node, nil
}

func GetNodeRelay(network string, relayedNodeAddr string) (models.Node, error) {
	collection, err := database.FetchRecords(database.NODES_TABLE_NAME)
	var relay models.Node
	if err != nil {
		if database.IsEmptyRecord(err) {
			return relay, nil
		}
		functions.PrintUserLog("", err.Error(), 2)
		return relay, err
	}
	for _, value := range collection {
		err := json.Unmarshal([]byte(value), &relay)
		if err != nil {
			functions.PrintUserLog("", err.Error(), 2)
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
