package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/skip2/go-qrcode"
)

func extClientHandlers(r *mux.Router) {

	r.HandleFunc("/api/extclients", securityCheck(false, http.HandlerFunc(getAllExtClients))).Methods("GET")
	r.HandleFunc("/api/extclients/{network}", securityCheck(false, http.HandlerFunc(getNetworkExtClients))).Methods("GET")
	r.HandleFunc("/api/extclients/{network}/{clientid}", securityCheck(false, http.HandlerFunc(getExtClient))).Methods("GET")
	r.HandleFunc("/api/extclients/{network}/{clientid}/{type}", securityCheck(false, http.HandlerFunc(getExtClientConf))).Methods("GET")
	r.HandleFunc("/api/extclients/{network}/{clientid}", securityCheck(false, http.HandlerFunc(updateExtClient))).Methods("PUT")
	r.HandleFunc("/api/extclients/{network}/{clientid}", securityCheck(false, http.HandlerFunc(deleteExtClient))).Methods("DELETE")
	r.HandleFunc("/api/extclients/{network}/{nodeid}", securityCheck(false, http.HandlerFunc(createExtClient))).Methods("POST")
}

func checkIngressExists(nodeID string) bool {
	node, err := logic.GetNodeByID(nodeID)
	if err != nil {
		return false
	}
	return node.IsIngressGateway == "yes"
}

//Gets all extclients associated with network, including pending extclients
func getNetworkExtClients(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var extclients []models.ExtClient
	var params = mux.Vars(r)
	extclients, err := logic.GetNetworkExtClients(params["network"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	//Returns all the extclients in JSON format
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(extclients)
}

//A separate function to get all extclients, not just extclients for a particular network.
//Not quite sure if this is necessary. Probably necessary based on front end but may want to review after iteration 1 if it's being used or not
func getAllExtClients(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	headerNetworks := r.Header.Get("networks")
	networksSlice := []string{}
	marshalErr := json.Unmarshal([]byte(headerNetworks), &networksSlice)
	if marshalErr != nil {
		returnErrorResponse(w, r, formatError(marshalErr, "internal"))
		return
	}
	clients := []models.ExtClient{}
	err := errors.New("Networks Error")
	if networksSlice[0] == ALL_NETWORK_ACCESS {
		clients, err = functions.GetAllExtClients()
		if err != nil && !database.IsEmptyRecord(err) {
			returnErrorResponse(w, r, formatError(err, "internal"))
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

//Get an individual extclient. Nothin fancy here folks.
func getExtClient(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	clientid := params["clientid"]
	network := params["network"]
	client, err := logic.GetExtClient(clientid, network)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(client)
}

//Get an individual extclient. Nothin fancy here folks.
func getExtClientConf(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	clientid := params["clientid"]
	networkid := params["network"]
	client, err := logic.GetExtClient(clientid, networkid)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	gwnode, err := logic.GetNodeByID(client.IngressGatewayID)
	if err != nil {
		logger.Log(1, r.Header.Get("user"), "Could not retrieve Ingress Gateway Node", client.IngressGatewayID)
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	network, err := logic.GetParentNetwork(client.Network)
	if err != nil {
		logger.Log(1, r.Header.Get("user"), "Could not retrieve Ingress Gateway Network", client.Network)
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	keepalive := ""
	if network.DefaultKeepalive != 0 {
		keepalive = "PersistentKeepalive = " + strconv.Itoa(int(network.DefaultKeepalive))
	}
	gwendpoint := gwnode.Endpoint + ":" + strconv.Itoa(int(gwnode.ListenPort))
	newAllowedIPs := network.AddressRange
	if egressGatewayRanges, err := logic.GetEgressRangesOnNetwork(&client); err == nil {
		for _, egressGatewayRange := range egressGatewayRanges {
			newAllowedIPs += "," + egressGatewayRange
		}
	}
	defaultDNS := ""
	if network.DefaultExtClientDNS != "" {
		defaultDNS = "DNS = " + network.DefaultExtClientDNS
	}
	config := fmt.Sprintf(`[Interface]
Address = %s
PrivateKey = %s
%s

[Peer]
PublicKey = %s
AllowedIPs = %s
Endpoint = %s
%s

`, client.Address+"/32",
		client.PrivateKey,
		defaultDNS,
		gwnode.PublicKey,
		newAllowedIPs,
		gwendpoint,
		keepalive)

	if params["type"] == "qr" {
		bytes, err := qrcode.Encode(config, qrcode.Medium, 220)
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(bytes)
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
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
			returnErrorResponse(w, r, formatError(err, "internal"))
		}
		return
	}
	logger.Log(2, r.Header.Get("user"), "retrieved ext client config")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(client)
}

/**
 * To create a extclient
 * Must have valid key and be unique
 */
func createExtClient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	networkName := params["network"]
	nodeid := params["nodeid"]
	ingressExists := checkIngressExists(nodeid)
	if !ingressExists {
		returnErrorResponse(w, r, formatError(errors.New("ingress does not exist"), "internal"))
		return
	}

	var extclient models.ExtClient
	extclient.Network = networkName
	extclient.IngressGatewayID = nodeid
	node, err := logic.GetNodeByID(nodeid)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	extclient.IngressGatewayEndpoint = node.Endpoint + ":" + strconv.FormatInt(int64(node.ListenPort), 10)
	err = json.NewDecoder(r.Body).Decode(&extclient)
	if err != nil && !errors.Is(err, io.EOF) {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	err = logic.CreateExtClient(&extclient)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	err = mq.PublishExtPeerUpdate(&node)
	if err != nil {
		logger.Log(1, "error setting ext peers on "+nodeid+": "+err.Error())
	}
}

func updateExtClient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	var newExtClient models.ExtClient
	var oldExtClient models.ExtClient
	_ = json.NewDecoder(r.Body).Decode(&newExtClient)

	key, err := logic.GetRecordKey(params["clientid"], params["network"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	data, err := database.FetchRecord(database.EXT_CLIENT_TABLE_NAME, key)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	if err = json.Unmarshal([]byte(data), &oldExtClient); err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	newclient, err := logic.UpdateExtClient(newExtClient.ClientID, params["network"], &oldExtClient)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "updated client", newExtClient.ClientID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newclient)
}

//Delete a extclient
//Pretty straightforward
func deleteExtClient(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params
	var params = mux.Vars(r)

	extclient, err := logic.GetExtClient(params["clientid"], params["network"])
	if err != nil {
		err = errors.New("Could not delete extclient " + params["clientid"])
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	ingressnode, err := logic.GetNodeByID(extclient.IngressGatewayID)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	err = logic.DeleteExtClient(params["network"], params["clientid"])

	if err != nil {
		err = errors.New("Could not delete extclient " + params["clientid"])
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	err = mq.PublishExtPeerUpdate(&ingressnode)
	if err != nil {
		logger.Log(1, "error setting ext peers on "+ingressnode.ID+": "+err.Error())
	}
	logger.Log(1, r.Header.Get("user"),
		"Deleted extclient client", params["clientid"], "from network", params["network"])
	returnSuccessResponse(w, r, params["clientid"]+" deleted.")
}
