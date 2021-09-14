package controller

import (
	"encoding/json"
	"errors"
	"net/http"
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
	node.RelayAddrs = relay.Addrs

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
	err = SetNodesDoNotPropagate("yes", node.Network, node.RelayAddrs)
	if err != nil {
		return node, err
	}

	if err = functions.NetworkNodesUpdatePullChanges(node.Network); err != nil {
		return models.Node{}, err
	}
	return node, nil
}

func SetNodesDoNotPropagate(yesOrno string, networkName string, addrs []string) error {

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
					node.DoNotPropogate = yesOrno
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
	empty := len(relay.Addrs) == 0
	if empty {
		err = errors.New("IP Ranges Cannot Be Empty")
	}
	return err
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
	err = SetNodesDoNotPropagate("yes", node.Network, node.RelayAddrs)
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