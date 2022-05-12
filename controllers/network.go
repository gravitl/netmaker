package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
)

// ALL_NETWORK_ACCESS - represents all networks
const ALL_NETWORK_ACCESS = "THIS_USER_HAS_ALL"

// NO_NETWORKS_PRESENT - represents no networks
const NO_NETWORKS_PRESENT = "THIS_USER_HAS_NONE"

func networkHandlers(r *mux.Router) {
	r.HandleFunc("/api/networks", securityCheck(false, http.HandlerFunc(getNetworks))).Methods("GET")
	r.HandleFunc("/api/networks", securityCheck(true, http.HandlerFunc(createNetwork))).Methods("POST")
	r.HandleFunc("/api/networks/{networkname}", securityCheck(false, http.HandlerFunc(getNetwork))).Methods("GET")
	r.HandleFunc("/api/networks/{networkname}", securityCheck(false, http.HandlerFunc(updateNetwork))).Methods("PUT")
	r.HandleFunc("/api/networks/{networkname}/nodelimit", securityCheck(true, http.HandlerFunc(updateNetworkNodeLimit))).Methods("PUT")
	r.HandleFunc("/api/networks/{networkname}", securityCheck(true, http.HandlerFunc(deleteNetwork))).Methods("DELETE")
	r.HandleFunc("/api/networks/{networkname}/keyupdate", securityCheck(true, http.HandlerFunc(keyUpdate))).Methods("POST")
	r.HandleFunc("/api/networks/{networkname}/keys", securityCheck(false, http.HandlerFunc(createAccessKey))).Methods("POST")
	r.HandleFunc("/api/networks/{networkname}/keys", securityCheck(false, http.HandlerFunc(getAccessKeys))).Methods("GET")
	r.HandleFunc("/api/networks/{networkname}/keys/{name}", securityCheck(false, http.HandlerFunc(deleteAccessKey))).Methods("DELETE")
	// ACLs
	r.HandleFunc("/api/networks/{networkname}/acls", securityCheck(true, http.HandlerFunc(updateNetworkACL))).Methods("PUT")
	r.HandleFunc("/api/networks/{networkname}/acls", securityCheck(true, http.HandlerFunc(getNetworkACL))).Methods("GET")
}

//simple get all networks function
func getNetworks(w http.ResponseWriter, r *http.Request) {

	headerNetworks := r.Header.Get("networks")
	networksSlice := []string{}
	marshalErr := json.Unmarshal([]byte(headerNetworks), &networksSlice)
	if marshalErr != nil {
		returnErrorResponse(w, r, formatError(marshalErr, "internal"))
		return
	}
	allnetworks := []models.Network{}
	var err error
	if networksSlice[0] == ALL_NETWORK_ACCESS {
		allnetworks, err = logic.GetNetworks()
		if err != nil && !database.IsEmptyRecord(err) {
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		}
	} else {
		for _, network := range networksSlice {
			netObject, parentErr := logic.GetParentNetwork(network)
			if parentErr == nil {
				allnetworks = append(allnetworks, netObject)
			}
		}
	}
	if !servercfg.IsDisplayKeys() {
		for i, net := range allnetworks {
			net.AccessKeys = logic.RemoveKeySensitiveInfo(net.AccessKeys)
			allnetworks[i] = net
		}
	}

	logger.Log(2, r.Header.Get("user"), "fetched networks.")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(allnetworks)
}

// Simple get network function
func getNetwork(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	netname := params["networkname"]
	network, err := logic.GetNetwork(netname)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	if !servercfg.IsDisplayKeys() {
		network.AccessKeys = logic.RemoveKeySensitiveInfo(network.AccessKeys)
	}
	logger.Log(2, r.Header.Get("user"), "fetched network", netname)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(network)
}

func keyUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	netname := params["networkname"]
	network, err := logic.KeyUpdate(netname)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(2, r.Header.Get("user"), "updated key on network", netname)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(network)
	nodes, err := logic.GetNetworkNodes(netname)
	if err != nil {
		logger.Log(2, "failed to retrieve network nodes for network", netname, err.Error())
		return
	}
	for _, node := range nodes {
		logger.Log(2, "updating node ", node.Name, " for a key update")
		if node.IsServer != "yes" {
			if err = mq.NodeUpdate(&node); err != nil {
				logger.Log(1, "failed to send update to node during a network wide key update", node.Name, node.ID, err.Error())
			}
		}
	}
}

// Update a network
func updateNetwork(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	var network models.Network
	netname := params["networkname"]

	network, err := logic.GetParentNetwork(netname)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	var newNetwork models.Network
	err = json.NewDecoder(r.Body).Decode(&newNetwork)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}

	if !servercfg.GetRce() {
		newNetwork.DefaultPostDown = network.DefaultPostDown
		newNetwork.DefaultPostUp = network.DefaultPostUp
	}

	rangeupdate4, rangeupdate6, localrangeupdate, holepunchupdate, err := logic.UpdateNetwork(&network, &newNetwork)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}

	if rangeupdate4 {
		err = logic.UpdateNetworkNodeAddresses(network.NetID)
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		}
	}
	if rangeupdate6 {
		err = logic.UpdateNetworkNodeAddresses6(network.NetID)
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		}
	}
	if localrangeupdate {
		err = logic.UpdateNetworkLocalAddresses(network.NetID)
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		}
	}
	if holepunchupdate {
		err = logic.UpdateNetworkHolePunching(network.NetID, newNetwork.DefaultUDPHolePunch)
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		}
	}
	if rangeupdate4 || rangeupdate6 || localrangeupdate || holepunchupdate {
		nodes, err := logic.GetNetworkNodes(network.NetID)
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		}
		for _, node := range nodes {
			if err = mq.NodeUpdate(&node); err != nil {
				logger.Log(1, "failed to send update to node during a network wide update", node.Name, node.ID, err.Error())
			}
		}
	}

	logger.Log(1, r.Header.Get("user"), "updated network", netname)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newNetwork)
}

func updateNetworkNodeLimit(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	var network models.Network
	netname := params["networkname"]
	network, err := logic.GetParentNetwork(netname)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	var networkChange models.Network

	_ = json.NewDecoder(r.Body).Decode(&networkChange)

	if networkChange.NodeLimit != 0 {
		network.NodeLimit = networkChange.NodeLimit
		data, err := json.Marshal(&network)
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "badrequest"))
			return
		}
		database.Insert(network.NetID, string(data), database.NETWORKS_TABLE_NAME)
		logger.Log(1, r.Header.Get("user"), "updated network node limit on", netname)
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(network)
}

func updateNetworkACL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	netname := params["networkname"]
	var networkACLChange acls.ACLContainer
	networkACLChange, err := networkACLChange.Get(acls.ContainerID(netname))
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	_ = json.NewDecoder(r.Body).Decode(&networkACLChange)
	newNetACL, err := networkACLChange.Save(acls.ContainerID(netname))
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "updated ACLs for network", netname)

	// send peer updates
	if servercfg.IsMessageQueueBackend() {
		serverNode, err := logic.GetNetworkServerLocal(netname)
		if err != nil {
			logger.Log(1, "failed to find server node after ACL update on", netname)
		} else {
			if err = logic.ServerUpdate(&serverNode, false); err != nil {
				logger.Log(1, "failed to update server node after ACL update on", netname)
			}
			if err = mq.PublishPeerUpdate(&serverNode); err != nil {
				logger.Log(0, "failed to publish peer update after ACL update on", netname)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newNetACL)
}

func getNetworkACL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	netname := params["networkname"]
	var networkACL acls.ACLContainer
	networkACL, err := networkACL.Get(acls.ContainerID(netname))
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(2, r.Header.Get("user"), "fetched acl for network", netname)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(networkACL)
}

// Delete a network
// Will stop you if  there's any nodes associated
func deleteNetwork(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	network := params["networkname"]
	err := logic.DeleteNetwork(network)
	if err != nil {
		errtype := "badrequest"
		if strings.Contains(err.Error(), "Node check failed") {
			errtype = "forbidden"
		}
		returnErrorResponse(w, r, formatError(err, errtype))
		return
	}
	logger.Log(1, r.Header.Get("user"), "deleted network", network)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode("success")
}

func createNetwork(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var network models.Network

	// we decode our body request params
	err := json.NewDecoder(r.Body).Decode(&network)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	if network.AddressRange == "" && network.AddressRange6 == "" {
		returnErrorResponse(w, r, formatError(fmt.Errorf("IPv4 or IPv6 CIDR required"), "badrequest"))
		return
	}

	network, err = logic.CreateNetwork(network)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}

	if servercfg.IsClientMode() != "off" {
		_, err := logic.ServerJoin(&network)
		if err != nil {
			logic.DeleteNetwork(network.NetID)
			if err == nil {
				err = errors.New("Failed to add server to network " + network.NetID)
			}
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		}
	}

	logger.Log(1, r.Header.Get("user"), "created network", network.NetID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(network)
}

// BEGIN KEY MANAGEMENT SECTION
func createAccessKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	var accesskey models.AccessKey
	//start here
	netname := params["networkname"]
	network, err := logic.GetParentNetwork(netname)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	err = json.NewDecoder(r.Body).Decode(&accesskey)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	key, err := logic.CreateAccessKey(accesskey, network)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "created access key", accesskey.Name, "on", netname)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(key)
}

// pretty simple get
func getAccessKeys(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	network := params["networkname"]
	keys, err := logic.GetKeys(network)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	if !servercfg.IsDisplayKeys() {
		keys = logic.RemoveKeySensitiveInfo(keys)
	}
	logger.Log(2, r.Header.Get("user"), "fetched access keys on network", network)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(keys)
}

// delete key. Has to do a little funky logic since it's not a collection item
func deleteAccessKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	keyname := params["name"]
	netname := params["networkname"]
	err := logic.DeleteKey(keyname, netname)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "deleted access key", keyname, "on network,", netname)
	w.WriteHeader(http.StatusOK)
}
