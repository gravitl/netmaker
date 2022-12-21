package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/pro"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/models/promodels"
	"github.com/gravitl/netmaker/mq"
	"github.com/skip2/go-qrcode"
)

func extClientHandlers(r *mux.Router) {

	r.HandleFunc("/api/extclients", logic.SecurityCheck(false, http.HandlerFunc(getAllExtClients))).Methods("GET")
	r.HandleFunc("/api/extclients/{network}", logic.SecurityCheck(false, http.HandlerFunc(getNetworkExtClients))).Methods("GET")
	r.HandleFunc("/api/extclients/{network}/{clientid}", logic.SecurityCheck(false, http.HandlerFunc(getExtClient))).Methods("GET")
	r.HandleFunc("/api/extclients/{network}/{clientid}/{type}", logic.NetUserSecurityCheck(false, true, http.HandlerFunc(getExtClientConf))).Methods("GET")
	r.HandleFunc("/api/extclients/{network}/{clientid}", logic.NetUserSecurityCheck(false, true, http.HandlerFunc(updateExtClient))).Methods("PUT")
	r.HandleFunc("/api/extclients/{network}/{clientid}", logic.NetUserSecurityCheck(false, true, http.HandlerFunc(deleteExtClient))).Methods("DELETE")
	r.HandleFunc("/api/extclients/{network}/{nodeid}", logic.NetUserSecurityCheck(false, true, checkFreeTierLimits(clients_l, http.HandlerFunc(createExtClient)))).Methods("POST")
}

func checkIngressExists(nodeID string) bool {
	node, err := logic.GetNodeByID(nodeID)
	if err != nil {
		return false
	}
	return node.IsIngressGateway == "yes"
}

// swagger:route GET /api/extclients/{network} ext_client getNetworkExtClients
//
// Get all extclients associated with network.
// Gets all extclients associated with network, including pending extclients.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: extClientSliceResponse
func getNetworkExtClients(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var extclients []models.ExtClient
	var params = mux.Vars(r)
	network := params["network"]
	extclients, err := logic.GetNetworkExtClients(network)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get ext clients for network [%s]: %v", network, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	//Returns all the extclients in JSON format
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(extclients)
}

// swagger:route GET /api/extclients ext_client getAllExtClients
//
// A separate function to get all extclients, not just extclients for a particular network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: extClientSliceResponse
//
// Not quite sure if this is necessary. Probably necessary based on front end but may
// want to review after iteration 1 if it's being used or not
func getAllExtClients(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	headerNetworks := r.Header.Get("networks")
	networksSlice := []string{}
	marshalErr := json.Unmarshal([]byte(headerNetworks), &networksSlice)
	if marshalErr != nil {
		logger.Log(0, "error unmarshalling networks: ",
			marshalErr.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(marshalErr, "internal"))
		return
	}
	clients := []models.ExtClient{}
	var err error
	if len(networksSlice) > 0 && networksSlice[0] == logic.ALL_NETWORK_ACCESS {
		clients, err = functions.GetAllExtClients()
		if err != nil && !database.IsEmptyRecord(err) {
			logger.Log(0, "failed to get all extclients: ", err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	} else {
		for _, network := range networksSlice {
			extclients, err := logic.GetNetworkExtClients(network)
			if err == nil {
				clients = append(clients, extclients...)
			}
		}
	}

	//Return all the extclients in JSON format
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(clients)
}

// swagger:route GET /api/extclients/{network}/{clientid} ext_client getExtClient
//
// Get an individual extclient.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: extClientResponse
func getExtClient(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	clientid := params["clientid"]
	network := params["network"]
	client, err := logic.GetExtClient(clientid, network)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), fmt.Sprintf("failed to get extclient for [%s] on network [%s]: %v",
			clientid, network, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(client)
}

// swagger:route GET /api/extclients/{network}/{clientid}/{type} ext_client getExtClientConf
//
// Get an individual extclient.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: extClientResponse
func getExtClientConf(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	clientid := params["clientid"]
	networkid := params["network"]
	client, err := logic.GetExtClient(clientid, networkid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), fmt.Sprintf("failed to get extclient for [%s] on network [%s]: %v",
			clientid, networkid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	gwnode, err := logic.GetNodeByID(client.IngressGatewayID)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get ingress gateway node [%s] info: %v", client.IngressGatewayID, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	network, err := logic.GetParentNetwork(client.Network)
	if err != nil {
		logger.Log(1, r.Header.Get("user"), "Could not retrieve Ingress Gateway Network", client.Network)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	addrString := client.Address
	if addrString != "" {
		addrString += "/32"
	}
	if client.Address6 != "" {
		if addrString != "" {
			addrString += ","
		}
		addrString += client.Address6 + "/128"
	}

	keepalive := ""
	if network.DefaultKeepalive != 0 {
		keepalive = "PersistentKeepalive = " + strconv.Itoa(int(network.DefaultKeepalive))
	}
	gwendpoint := gwnode.Endpoint + ":" + strconv.Itoa(int(gwnode.ListenPort))
	newAllowedIPs := network.AddressRange
	if newAllowedIPs != "" && network.AddressRange6 != "" {
		newAllowedIPs += ","
	}
	if network.AddressRange6 != "" {
		newAllowedIPs += network.AddressRange6
	}
	if egressGatewayRanges, err := logic.GetEgressRangesOnNetwork(&client); err == nil {
		for _, egressGatewayRange := range egressGatewayRanges {
			newAllowedIPs += "," + egressGatewayRange
		}
	}
	defaultDNS := ""
	if network.DefaultExtClientDNS != "" {
		defaultDNS = "DNS = " + network.DefaultExtClientDNS
	}

	defaultMTU := 1420
	if gwnode.MTU != 0 {
		defaultMTU = int(gwnode.MTU)
	}
	config := fmt.Sprintf(`[Interface]
Address = %s
PrivateKey = %s
MTU = %d
%s

[Peer]
PublicKey = %s
AllowedIPs = %s
Endpoint = %s
%s

`, addrString,
		client.PrivateKey,
		defaultMTU,
		defaultDNS,
		gwnode.PublicKey,
		newAllowedIPs,
		gwendpoint,
		keepalive)

	if params["type"] == "qr" {
		bytes, err := qrcode.Encode(config, qrcode.Medium, 220)
		if err != nil {
			logger.Log(1, r.Header.Get("user"), "failed to encode qr code: ", err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(bytes)
		if err != nil {
			logger.Log(1, r.Header.Get("user"), "response writer error (qr) ", err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		return
	}

	if params["type"] == "file" {
		name := client.ClientID + ".conf"
		w.Header().Set("Content-Type", "application/config")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
		w.WriteHeader(http.StatusOK)
		_, err := fmt.Fprint(w, config)
		if err != nil {
			logger.Log(1, r.Header.Get("user"), "response writer error (file) ", err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		}
		return
	}
	logger.Log(2, r.Header.Get("user"), "retrieved ext client config")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(client)
}

// swagger:route POST /api/extclients/{network}/{nodeid} ext_client createExtClient
//
// Create an individual extclient.  Must have valid key and be unique.
//
//			Schemes: https
//
//			Security:
//	  		oauth
func createExtClient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	networkName := params["network"]
	nodeid := params["nodeid"]

	ingressExists := checkIngressExists(nodeid)
	if !ingressExists {
		err := errors.New("ingress does not exist")
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to create extclient on network [%s]: %v", networkName, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	var extclient models.ExtClient
	var CustomExtClient models.CustomExtClient

	err := json.NewDecoder(r.Body).Decode(&CustomExtClient)

	if err == nil {
		if CustomExtClient.ClientID != "" && !validName(CustomExtClient.ClientID) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errInvalidExtClientID, "badrequest"))
			return
		}
		extclient.ClientID = CustomExtClient.ClientID
	}

	extclient.Network = networkName
	extclient.IngressGatewayID = nodeid
	node, err := logic.GetNodeByID(nodeid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get ingress gateway node [%s] info: %v", nodeid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	extclient.IngressGatewayEndpoint = node.Endpoint + ":" + strconv.FormatInt(int64(node.ListenPort), 10)

	extclient.Enabled = true
	parentNetwork, err := logic.GetNetwork(networkName)
	if err == nil { // check if parent network default ACL is enabled (yes) or not (no)
		extclient.Enabled = parentNetwork.DefaultACL == "yes"
	}
	// check pro settings

	err = logic.CreateExtClient(&extclient)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to create new ext client on network [%s]: %v", networkName, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	var isAdmin bool
	if r.Header.Get("ismaster") != "yes" {
		userID := r.Header.Get("user")
		if isAdmin, err = checkProClientAccess(userID, extclient.ClientID, &parentNetwork); err != nil {
			logger.Log(0, userID, "attempted to create a client on network", networkName, "but they lack access")
			logic.DeleteExtClient(networkName, extclient.ClientID)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		if !isAdmin {
			if err = pro.AssociateNetworkUserClient(userID, networkName, extclient.ClientID); err != nil {
				logger.Log(0, "failed to associate client", extclient.ClientID, "to user", userID)
			}
			extclient.OwnerID = userID
			if _, err := logic.UpdateExtClient(extclient.ClientID, extclient.Network, extclient.Enabled, &extclient); err != nil {
				logger.Log(0, "failed to add owner id", userID, "to client", extclient.ClientID)
			}
		}
	}

	logger.Log(0, r.Header.Get("user"), "created new ext client on network", networkName)
	w.WriteHeader(http.StatusOK)
	err = mq.PublishExtPeerUpdate(&node)
	if err != nil {
		logger.Log(1, "error setting ext peers on "+nodeid+": "+err.Error())
	}
}

// swagger:route PUT /api/extclients/{network}/{clientid} ext_client updateExtClient
//
// Update an individual extclient.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: extClientResponse
func updateExtClient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	var newExtClient models.ExtClient
	var oldExtClient models.ExtClient
	err := json.NewDecoder(r.Body).Decode(&newExtClient)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	clientid := params["clientid"]
	network := params["network"]
	key, err := logic.GetRecordKey(clientid, network)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get record key for client [%s], network [%s]: %v",
				clientid, network, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if !validName(newExtClient.ClientID) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errInvalidExtClientID, "badrequest"))
		return
	}
	data, err := database.FetchRecord(database.EXT_CLIENT_TABLE_NAME, key)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to fetch  ext client record key [%s] from db for client [%s], network [%s]: %v",
				key, clientid, network, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if err = json.Unmarshal([]byte(data), &oldExtClient); err != nil {
		logger.Log(0, "error unmarshalling extclient: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	// == PRO ==
	networkName := params["network"]
	var changedID = newExtClient.ClientID != oldExtClient.ClientID
	if r.Header.Get("ismaster") != "yes" {
		userID := r.Header.Get("user")
		_, doesOwn := doesUserOwnClient(userID, params["clientid"], networkName)
		if !doesOwn {
			logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("user not permitted"), "internal"))
			return
		}
	}

	if changedID && oldExtClient.OwnerID != "" {
		if err := pro.DissociateNetworkUserClient(oldExtClient.OwnerID, networkName, oldExtClient.ClientID); err != nil {
			logger.Log(0, "failed to dissociate client", oldExtClient.ClientID, "from user", oldExtClient.OwnerID)
		}
		if err := pro.AssociateNetworkUserClient(oldExtClient.OwnerID, networkName, newExtClient.ClientID); err != nil {
			logger.Log(0, "failed to associate client", newExtClient.ClientID, "to user", oldExtClient.OwnerID)
		}
	}
	// == END PRO ==

	var changedEnabled = newExtClient.Enabled != oldExtClient.Enabled // indicates there was a change in enablement

	newclient, err := logic.UpdateExtClient(newExtClient.ClientID, params["network"], newExtClient.Enabled, &oldExtClient)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to update ext client [%s], network [%s]: %v",
				clientid, network, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(0, r.Header.Get("user"), "updated ext client", newExtClient.ClientID)
	if changedEnabled { // need to send a peer update to the ingress node as enablement of one of it's clients has changed
		if ingressNode, err := logic.GetNodeByID(newclient.IngressGatewayID); err == nil {
			if err = mq.PublishExtPeerUpdate(&ingressNode); err != nil {
				logger.Log(1, "error setting ext peers on", ingressNode.ID, ":", err.Error())
			}
		}
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newclient)
}

// swagger:route DELETE /api/extclients/{network}/{clientid} ext_client deleteExtClient
//
// Delete an individual extclient.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: successResponse
func deleteExtClient(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params
	var params = mux.Vars(r)
	clientid := params["clientid"]
	network := params["network"]
	extclient, err := logic.GetExtClient(clientid, network)
	if err != nil {
		err = errors.New("Could not delete extclient " + params["clientid"])
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to delete extclient [%s],network [%s]: %v", clientid, network, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	ingressnode, err := logic.GetNodeByID(extclient.IngressGatewayID)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get ingress gateway node [%s] info: %v", extclient.IngressGatewayID, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	// == PRO ==
	if r.Header.Get("ismaster") != "yes" {
		userID, clientID, networkName := r.Header.Get("user"), params["clientid"], params["network"]
		_, doesOwn := doesUserOwnClient(userID, clientID, networkName)
		if !doesOwn {
			logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("user not permitted"), "internal"))
			return
		}
	}

	if extclient.OwnerID != "" {
		if err = pro.DissociateNetworkUserClient(extclient.OwnerID, extclient.Network, extclient.ClientID); err != nil {
			logger.Log(0, "failed to dissociate client", extclient.ClientID, "from user", extclient.OwnerID)
		}
	}

	// == END PRO ==

	err = logic.DeleteExtClient(params["network"], params["clientid"])
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to delete extclient [%s],network [%s]: %v", clientid, network, err))
		err = errors.New("Could not delete extclient " + params["clientid"])
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	err = mq.PublishExtPeerUpdate(&ingressnode)
	if err != nil {
		logger.Log(1, "error setting ext peers on "+ingressnode.ID+": "+err.Error())
	}

	logger.Log(0, r.Header.Get("user"),
		"Deleted extclient client", params["clientid"], "from network", params["network"])
	logic.ReturnSuccessResponse(w, r, params["clientid"]+" deleted.")
}

func checkProClientAccess(username, clientID string, network *models.Network) (bool, error) {
	u, err := logic.GetUser(username)
	if err != nil {
		return false, err
	}
	if u.IsAdmin {
		return true, nil
	}

	netUser, err := pro.GetNetworkUser(network.NetID, promodels.NetworkUserID(u.UserName))
	if err != nil {
		return false, err
	}

	if netUser.AccessLevel == pro.NET_ADMIN {
		return false, nil
	}

	if netUser.AccessLevel == pro.NO_ACCESS {
		return false, fmt.Errorf("user does not have access")
	}

	if !(len(netUser.Clients) < netUser.ClientLimit) {
		return false, fmt.Errorf("user can not create more clients")
	}

	if netUser.AccessLevel < pro.NO_ACCESS {
		netUser.Clients = append(netUser.Clients, clientID)
		if err = pro.UpdateNetworkUser(network.NetID, netUser); err != nil {
			return false, err
		}
	}
	return false, nil
}

// checks if net user owns an ext client or is an admin
func doesUserOwnClient(username, clientID, network string) (bool, bool) {
	u, err := logic.GetUser(username)
	if err != nil {
		return false, false
	}
	if u.IsAdmin {
		return true, true
	}

	netUser, err := pro.GetNetworkUser(network, promodels.NetworkUserID(u.UserName))
	if err != nil {
		return false, false
	}

	if netUser.AccessLevel == pro.NET_ADMIN {
		return false, true
	}

	return false, logic.StringSliceContains(netUser.Clients, clientID)
}
