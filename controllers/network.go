package controller

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/serverctl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
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
	r.HandleFunc("/api/networks/{networkname}/keyupdate", securityCheck(false, http.HandlerFunc(keyUpdate))).Methods("POST")
	r.HandleFunc("/api/networks/{networkname}/keys", securityCheck(false, http.HandlerFunc(createAccessKey))).Methods("POST")
	r.HandleFunc("/api/networks/{networkname}/keys", securityCheck(false, http.HandlerFunc(getAccessKeys))).Methods("GET")
	r.HandleFunc("/api/networks/{networkname}/keys/{name}", securityCheck(false, http.HandlerFunc(deleteAccessKey))).Methods("DELETE")
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
	err := errors.New("Networks Error")
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
		logger.Log(3, "updating node ", node.Name, " for a key update")
		runUpdates(&node, true)
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

	rangeupdate, localrangeupdate, holepunchupdate, err := logic.UpdateNetwork(&network, &newNetwork)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}

	// if newNetwork.IsDualStack != currentNetwork.IsDualStack && newNetwork.IsDualStack == "no" {
	// 	// Remove IPv6 address from network nodes
	// 	RemoveNetworkNodeIPv6Addresses(currentNetwork.NetID)
	// }

	if rangeupdate {
		err = logic.UpdateNetworkNodeAddresses(network.NetID)
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
	if rangeupdate || localrangeupdate || holepunchupdate {
		nodes, err := logic.GetNetworkNodes(network.NetID)
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		}
		var serverAddrs = make([]models.ServerAddr, 0)
		if rangeupdate {
			serverAddrs = preCalculateServerAddrs(network.NetID)
		}
		leaderServerNode, err := logic.GetNetworkServerLeader(netname)
		if err != nil {
			logger.Log(1, "failed to update peers for server node address on network", netname)
		}

		for _, node := range nodes {
			if node.IsServer != "yes" {
				if rangeupdate {
					applyServerAddr(&node, serverAddrs, network)
					var rangeUpdate models.RangeUpdate
					rangeUpdate.Node = node
					rangeUpdate.Peers.Network = node.Network
					rangeUpdate.Peers.ServerAddrs = serverAddrs
					var peer wgtypes.PeerConfig
					peer.PublicKey, err = wgtypes.ParseKey(leaderServerNode.PublicKey)
					if err != nil {
						returnErrorResponse(w, r, formatError(err, "internal"))
						return
					}
					peer.ReplaceAllowedIPs = true
					for _, server := range serverAddrs {
						if server.IsLeader {
							_, address, err := net.ParseCIDR(server.Address + "/32")
							if err != nil {
								returnErrorResponse(w, r, formatError(err, "internal"))
								return
							}
							peer.AllowedIPs = append(peer.AllowedIPs, *address)
						}
					}
					rangeUpdate.Peers.Peers = append(rangeUpdate.Peers.Peers, peer)
					if err := mq.PublishRangeUpdate(&rangeUpdate); err != nil {
						returnErrorResponse(w, r, formatError(err, "internal"))
						return
					}
					if err := mq.NodeUpdate(&node); err != nil {
						logger.Log(1, "could not update range when network", netname, "changed cidr for node", node.Name, node.ID, err.Error())
					}
				}
			}
		}
	}
	logger.Log(1, r.Header.Get("user"), "updated network", netname)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newNetwork)
	//currentServerNode, err := logic.GetNetworkServerLocal(netname)
	//if err != nil {
	//	logger.Log(1, "failed to update peers for server node address on network", netname)
	//} else {
	//	runUpdates(&currentServerNode, true)
	//}
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

	err = logic.CreateNetwork(network)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}

	if servercfg.IsClientMode() != "off" {
		var node models.Node
		node, err = logic.ServerJoin(&network)
		if err != nil {
			logic.DeleteNetwork(network.NetID)
			if err == nil {
				err = errors.New("Failed to add server to network " + network.DisplayName)
			}
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		}
		getServerAddrs(&node)
	}

	logger.Log(1, r.Header.Get("user"), "created network", network.NetID)
	w.WriteHeader(http.StatusOK)
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

// used for network address changes
func applyServerAddr(node *models.Node, serverAddrs []models.ServerAddr, network models.Network) {
	node.NetworkSettings = network
	node.NetworkSettings.DefaultServerAddrs = serverAddrs
}

func preCalculateServerAddrs(netname string) []models.ServerAddr {
	var serverAddrs = make([]models.ServerAddr, 0)
	serverNodes := logic.GetServerNodes(netname)
	if len(serverNodes) == 0 {
		if err := serverctl.SyncServerNetwork(netname); err != nil {
			return serverAddrs
		}
	}

	address, err := logic.UniqueAddressServer(netname)
	if err != nil {
		return serverAddrs
	}
	for i := range serverNodes {
		addrParts := strings.Split(address, ".")                      // get the numbers
		lastNum, lastErr := strconv.Atoi(addrParts[len(addrParts)-1]) // get the last number as an int
		if lastErr == nil {
			lastNum = lastNum - i
			addrParts[len(addrParts)-1] = strconv.Itoa(lastNum)
			serverAddrs = append(serverAddrs, models.ServerAddr{
				IsLeader: logic.IsLeader(&serverNodes[i]),
				Address:  strings.Join(addrParts, "."),
			})
		}
	}
	return serverAddrs
}
