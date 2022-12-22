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

func networkHandlers(r *mux.Router) {
	r.HandleFunc("/api/networks", logic.SecurityCheck(false, http.HandlerFunc(getNetworks))).Methods("GET")
	r.HandleFunc("/api/networks", logic.SecurityCheck(true, checkFreeTierLimits(networks_l, http.HandlerFunc(createNetwork)))).Methods("POST")
	r.HandleFunc("/api/networks/{networkname}", logic.SecurityCheck(false, http.HandlerFunc(getNetwork))).Methods("GET")
	r.HandleFunc("/api/networks/{networkname}", logic.SecurityCheck(false, http.HandlerFunc(updateNetwork))).Methods("PUT")
	r.HandleFunc("/api/networks/{networkname}", logic.SecurityCheck(true, http.HandlerFunc(deleteNetwork))).Methods("DELETE")
	r.HandleFunc("/api/networks/{networkname}/keyupdate", logic.SecurityCheck(true, http.HandlerFunc(keyUpdate))).Methods("POST")
	r.HandleFunc("/api/networks/{networkname}/keys", logic.SecurityCheck(false, http.HandlerFunc(createAccessKey))).Methods("POST")
	r.HandleFunc("/api/networks/{networkname}/keys", logic.SecurityCheck(false, http.HandlerFunc(getAccessKeys))).Methods("GET")
	r.HandleFunc("/api/networks/{networkname}/keys/{name}", logic.SecurityCheck(false, http.HandlerFunc(deleteAccessKey))).Methods("DELETE")
	// ACLs
	r.HandleFunc("/api/networks/{networkname}/acls", logic.SecurityCheck(true, http.HandlerFunc(updateNetworkACL))).Methods("PUT")
	r.HandleFunc("/api/networks/{networkname}/acls", logic.SecurityCheck(true, http.HandlerFunc(getNetworkACL))).Methods("GET")
}

// swagger:route GET /api/networks networks getNetworks
//
// Lists all networks.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: getNetworksSliceResponse
func getNetworks(w http.ResponseWriter, r *http.Request) {

	headerNetworks := r.Header.Get("networks")
	networksSlice := []string{}
	marshalErr := json.Unmarshal([]byte(headerNetworks), &networksSlice)
	if marshalErr != nil {
		logger.Log(0, r.Header.Get("user"), "error unmarshalling networks: ",
			marshalErr.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(marshalErr, "badrequest"))
		return
	}
	allnetworks := []models.Network{}
	var err error
	if len(networksSlice) > 0 && networksSlice[0] == logic.ALL_NETWORK_ACCESS {
		allnetworks, err = logic.GetNetworks()
		if err != nil && !database.IsEmptyRecord(err) {
			logger.Log(0, r.Header.Get("user"), "failed to fetch networks: ", err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
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

// swagger:route GET /api/networks/{networkname} networks getNetwork
//
// Get a network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: networkBodyResponse
func getNetwork(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	netname := params["networkname"]
	network, err := logic.GetNetwork(netname)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), fmt.Sprintf("failed to fetch network [%s] info: %v",
			netname, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if !servercfg.IsDisplayKeys() {
		network.AccessKeys = logic.RemoveKeySensitiveInfo(network.AccessKeys)
	}
	logger.Log(2, r.Header.Get("user"), "fetched network", netname)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(network)
}

// swagger:route POST /api/networks/{networkname}/keyupdate networks keyUpdate
//
// Update keys for a network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: networkBodyResponse
func keyUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	netname := params["networkname"]
	network, err := logic.KeyUpdate(netname)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), fmt.Sprintf("failed to update keys for network [%s]: %v",
			netname, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(2, r.Header.Get("user"), "updated key on network", netname)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(network)
	nodes, err := logic.GetNetworkNodes(netname)
	if err != nil {
		logger.Log(0, "failed to retrieve network nodes for network", netname, err.Error())
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

// swagger:route PUT /api/networks/{networkname} networks updateNetwork
//
// Update a network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: networkBodyResponse
func updateNetwork(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	var network models.Network
	netname := params["networkname"]

	network, err := logic.GetParentNetwork(netname)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to get network info: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	var newNetwork models.Network
	err = json.NewDecoder(r.Body).Decode(&newNetwork)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	if !servercfg.GetRce() {
		newNetwork.DefaultPostDown = network.DefaultPostDown
		newNetwork.DefaultPostUp = network.DefaultPostUp
	}

	rangeupdate4, rangeupdate6, localrangeupdate, holepunchupdate, groupsDelta, userDelta, err := logic.UpdateNetwork(&network, &newNetwork)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update network: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	if len(groupsDelta) > 0 {
		for _, g := range groupsDelta {
			users, err := logic.GetGroupUsers(g)
			if err == nil {
				for _, user := range users {
					logic.AdjustNetworkUserPermissions(&user, &newNetwork)
				}
			}
		}
	}
	if len(userDelta) > 0 {
		for _, uname := range userDelta {
			user, err := logic.GetReturnUser(uname)
			if err == nil {
				logic.AdjustNetworkUserPermissions(&user, &newNetwork)
			}
		}
	}
	if rangeupdate4 {
		err = logic.UpdateNetworkNodeAddresses(network.NetID)
		if err != nil {
			logger.Log(0, r.Header.Get("user"),
				fmt.Sprintf("failed to update network [%s] ipv4 addresses: %v",
					network.NetID, err.Error()))
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	}
	if rangeupdate6 {
		err = logic.UpdateNetworkNodeAddresses6(network.NetID)
		if err != nil {
			logger.Log(0, r.Header.Get("user"),
				fmt.Sprintf("failed to update network [%s] ipv6 addresses: %v",
					network.NetID, err.Error()))
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	}
	if localrangeupdate {
		err = logic.UpdateNetworkLocalAddresses(network.NetID)
		if err != nil {
			logger.Log(0, r.Header.Get("user"),
				fmt.Sprintf("failed to update network [%s] local addresses: %v",
					network.NetID, err.Error()))
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	}
	if holepunchupdate {
		err = logic.UpdateNetworkHolePunching(network.NetID, newNetwork.DefaultUDPHolePunch)
		if err != nil {
			logger.Log(0, r.Header.Get("user"),
				fmt.Sprintf("failed to update network [%s] hole punching: %v",
					network.NetID, err.Error()))
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	}
	if rangeupdate4 || rangeupdate6 || localrangeupdate || holepunchupdate {
		nodes, err := logic.GetNetworkNodes(network.NetID)
		if err != nil {
			logger.Log(0, r.Header.Get("user"),
				fmt.Sprintf("failed to get network [%s] nodes: %v",
					network.NetID, err.Error()))
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		for _, node := range nodes {
			runUpdates(&node, true)
		}
	}

	logger.Log(1, r.Header.Get("user"), "updated network", netname)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newNetwork)
}

// swagger:route PUT /api/networks/{networkname}/acls networks updateNetworkACL
//
// Update a network ACL (Access Control List).
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: aclContainerResponse
func updateNetworkACL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	netname := params["networkname"]
	var networkACLChange acls.ACLContainer
	networkACLChange, err := networkACLChange.Get(acls.ContainerID(netname))
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to fetch ACLs for network [%s]: %v", netname, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	err = json.NewDecoder(r.Body).Decode(&networkACLChange)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	newNetACL, err := networkACLChange.Save(acls.ContainerID(netname))
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to update ACLs for network [%s]: %v", netname, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
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
			if err = mq.PublishPeerUpdate(&serverNode, false); err != nil {
				logger.Log(0, "failed to publish peer update after ACL update on", netname)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newNetACL)
}

// swagger:route GET /api/networks/{networkname}/acls networks getNetworkACL
//
// Get a network ACL (Access Control List).
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: aclContainerResponse
func getNetworkACL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	netname := params["networkname"]
	var networkACL acls.ACLContainer
	networkACL, err := networkACL.Get(acls.ContainerID(netname))
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to fetch ACLs for network [%s]: %v", netname, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(2, r.Header.Get("user"), "fetched acl for network", netname)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(networkACL)
}

// swagger:route DELETE /api/networks/{networkname} networks deleteNetwork
//
// Delete a network.  Will not delete if there are any nodes that belong to the network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: stringJSONResponse
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
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to delete network [%s]: %v", network, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, errtype))
		return
	}
	// Deletes the network role from MQ
	event := mq.MqDynsecPayload{
		Commands: []mq.MqDynSecCmd{
			{
				Command:  mq.DeleteRoleCmd,
				RoleName: network,
			},
		},
	}

	if err := mq.PublishEventToDynSecTopic(event); err != nil {
		logger.Log(0, fmt.Sprintf("failed to send DynSec command [%v]: %v",
			event.Commands, err.Error()))
	}
	logger.Log(1, r.Header.Get("user"), "deleted network", network)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode("success")
}

// swagger:route POST /api/networks networks createNetwork
//
// Create a network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: networkBodyResponse
func createNetwork(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var network models.Network

	// we decode our body request params
	err := json.NewDecoder(r.Body).Decode(&network)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	if network.AddressRange == "" && network.AddressRange6 == "" {
		err := errors.New("IPv4 or IPv6 CIDR required")
		logger.Log(0, r.Header.Get("user"), "failed to create network: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	network, err = logic.CreateNetwork(network)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to create network: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	// Create Role with acls for the network
	event := mq.MqDynsecPayload{
		Commands: []mq.MqDynSecCmd{
			{
				Command:  mq.CreateRoleCmd,
				RoleName: network.NetID,
				Textname: "Network wide role with Acls for nodes",
				Acls:     mq.FetchNetworkAcls(network.NetID),
			},
		},
	}

	if err = mq.PublishEventToDynSecTopic(event); err != nil {
		logger.Log(0, fmt.Sprintf("failed to send DynSec command [%v]: %v",
			event.Commands, err.Error()))
	}

	if servercfg.IsClientMode() != "off" {
		if _, err = logic.ServerJoin(&network); err != nil {
			_ = logic.DeleteNetwork(network.NetID)
			logger.Log(0, r.Header.Get("user"), "failed to create network: ",
				err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	}

	logger.Log(1, r.Header.Get("user"), "created network", network.NetID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(network)
}

// swagger:route POST /api/networks/{networkname}/keys networks createAccessKey
//
// Create a network access key.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: accessKeyBodyResponse
//
// BEGIN KEY MANAGEMENT SECTION
func createAccessKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	var accesskey models.AccessKey
	// start here
	netname := params["networkname"]
	network, err := logic.GetParentNetwork(netname)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to get network info: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	err = json.NewDecoder(r.Body).Decode(&accesskey)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	key, err := logic.CreateAccessKey(accesskey, network)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to create access key: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	// do not allow access key creations view API with user names
	if _, err = logic.GetUser(key.Name); err == nil {
		logger.Log(0, "access key creation with invalid name attempted by", r.Header.Get("user"))
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("cannot create access key with user name"), "badrequest"))
		logic.DeleteKey(key.Name, network.NetID)
		return
	}

	logger.Log(1, r.Header.Get("user"), "created access key", accesskey.Name, "on", netname)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(key)
}

// swagger:route GET /api/networks/{networkname}/keys networks getAccessKeys
//
// Get network access keys for a network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: accessKeySliceBodyResponse
func getAccessKeys(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	network := params["networkname"]
	keys, err := logic.GetKeys(network)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), fmt.Sprintf("failed to get keys for network [%s]: %v",
			network, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if !servercfg.IsDisplayKeys() {
		keys = logic.RemoveKeySensitiveInfo(keys)
	}
	logger.Log(2, r.Header.Get("user"), "fetched access keys on network", network)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(keys)
}

// swagger:route DELETE /api/networks/{networkname}/keys/{name} networks deleteAccessKey
//
// Delete a network access key.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200:
//				*: stringJSONResponse
//
// delete key. Has to do a little funky logic since it's not a collection item
func deleteAccessKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	keyname := params["name"]
	netname := params["networkname"]
	err := logic.DeleteKey(keyname, netname)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), fmt.Sprintf("failed to delete key [%s] for network [%s]: %v",
			keyname, netname, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "deleted access key", keyname, "on network,", netname)
	w.WriteHeader(http.StatusOK)
}
