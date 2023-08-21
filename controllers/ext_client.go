package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"

	"github.com/gravitl/netmaker/models"

	"github.com/gravitl/netmaker/mq"
	"github.com/skip2/go-qrcode"
	"golang.org/x/exp/slog"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func extClientHandlers(r *mux.Router) {

	r.HandleFunc("/api/extclients", logic.SecurityCheck(false, http.HandlerFunc(getAllExtClients))).Methods(http.MethodGet)
	r.HandleFunc("/api/extclients/{network}", logic.SecurityCheck(false, http.HandlerFunc(getNetworkExtClients))).Methods(http.MethodGet)
	r.HandleFunc("/api/extclients/{network}/{clientid}", logic.SecurityCheck(false, http.HandlerFunc(getExtClient))).Methods(http.MethodGet)
	r.HandleFunc("/api/extclients/{network}/{clientid}/{type}", getExtClientConf).Methods(http.MethodGet)
	r.HandleFunc("/api/extclients/{network}/{clientid}", updateExtClient).Methods(http.MethodPut)
	r.HandleFunc("/api/extclients/{network}/{clientid}", deleteExtClient).Methods(http.MethodDelete)
	r.HandleFunc("/api/extclients/{network}/{nodeid}", createExtClient).Methods(http.MethodPost)
}

func checkIngressExists(nodeID string) bool {
	node, err := logic.GetNodeByID(nodeID)
	if err != nil {
		return false
	}
	return node.IsIngressGateway
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
		clients, err = logic.GetAllExtClients()
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
	logic.SortExtClient(clients[:])
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
	host, err := logic.GetHost(gwnode.HostID.String())
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get host for ingress gateway node [%s] info: %v", client.IngressGatewayID, err))
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
	gwendpoint := ""
	if host.EndpointIP.To4() == nil {
		gwendpoint = fmt.Sprintf("[%s]:%d", host.EndpointIP.String(), host.ListenPort)
	} else {
		gwendpoint = fmt.Sprintf("%s:%d", host.EndpointIP.String(), host.ListenPort)
	}
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
	if client.DNS != "" {
		defaultDNS = "DNS = " + client.DNS
	} else if gwnode.IngressDNS != "" {
		defaultDNS = "DNS = " + gwnode.IngressDNS
	}

	defaultMTU := 1420
	if host.MTU != 0 {
		defaultMTU = host.MTU
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
		host.PublicKey,
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
	nodeid := params["nodeid"]

	ingressExists := checkIngressExists(nodeid)
	if !ingressExists {
		err := errors.New("ingress does not exist")
		slog.Error("failed to create extclient", "user", r.Header.Get("user"), "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	var customExtClient models.CustomExtClient

	if err := json.NewDecoder(r.Body).Decode(&customExtClient); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if err := validateCustomExtClient(&customExtClient, true); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	extclient := logic.UpdateExtClient(&models.ExtClient{}, &customExtClient)

	extclient.IngressGatewayID = nodeid
	node, err := logic.GetNodeByID(nodeid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get ingress gateway node [%s] info: %v", nodeid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	extclient.Network = node.Network
	host, err := logic.GetHost(node.HostID.String())
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get ingress gateway host for node [%s] info: %v", nodeid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	listenPort := logic.GetPeerListenPort(host)
	extclient.IngressGatewayEndpoint = fmt.Sprintf("%s:%d", host.EndpointIP.String(), listenPort)
	extclient.Enabled = true
	parentNetwork, err := logic.GetNetwork(node.Network)
	if err == nil { // check if parent network default ACL is enabled (yes) or not (no)
		extclient.Enabled = parentNetwork.DefaultACL == "yes"
	}

	if err := logic.SetClientDefaultACLs(&extclient); err != nil {
		slog.Error("failed to set default acls for extclient", "user", r.Header.Get("user"), "network", node.Network, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	if err = logic.CreateExtClient(&extclient); err != nil {
		slog.Error("failed to create extclient", "user", r.Header.Get("user"), "network", node.Network, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	slog.Info("created extclient", "user", r.Header.Get("user"), "network", node.Network, "clientid", extclient.ClientID)
	w.WriteHeader(http.StatusOK)
	go func() {
		if err := mq.PublishPeerUpdate(); err != nil {
			logger.Log(1, "error setting ext peers on "+nodeid+": "+err.Error())
		}
		if err := mq.PublishExtCLientDNS(&extclient); err != nil {
			logger.Log(1, "error publishing extclient dns", err.Error())
		}
	}()
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

	var update models.CustomExtClient
	//var oldExtClient models.ExtClient
	var sendPeerUpdate bool
	err := json.NewDecoder(r.Body).Decode(&update)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	clientid := params["clientid"]
	oldExtClient, err := logic.GetExtClientByName(clientid)
	if err != nil {
		slog.Error("failed to retrieve extclient", "user", r.Header.Get("user"), "id", clientid, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if oldExtClient.ClientID == update.ClientID {
		if err := validateCustomExtClient(&update, false); err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	} else {
		if err := validateCustomExtClient(&update, true); err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	}
	var changedID = update.ClientID != oldExtClient.ClientID

	if len(update.DeniedACLs) != len(oldExtClient.DeniedACLs) {
		sendPeerUpdate = true
		logic.SetClientACLs(&oldExtClient, update.DeniedACLs)
	}

	if update.Enabled != oldExtClient.Enabled {
		sendPeerUpdate = true
	}
	newclient := logic.UpdateExtClient(&oldExtClient, &update)
	if err := logic.DeleteExtClient(oldExtClient.Network, oldExtClient.ClientID); err != nil {

		slog.Error("failed to delete ext client", "user", r.Header.Get("user"), "id", oldExtClient.ClientID, "network", oldExtClient.Network, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if err := logic.SaveExtClient(&newclient); err != nil {
		slog.Error("failed to save ext client", "user", r.Header.Get("user"), "id", newclient.ClientID, "network", newclient.Network, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(0, r.Header.Get("user"), "updated ext client", update.ClientID)
	if sendPeerUpdate { // need to send a peer update to the ingress node as enablement of one of it's clients has changed
		if ingressNode, err := logic.GetNodeByID(newclient.IngressGatewayID); err == nil {
			if err = mq.PublishPeerUpdate(); err != nil {
				logger.Log(1, "error setting ext peers on", ingressNode.ID.String(), ":", err.Error())
			}
		}
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newclient)
	if changedID {
		go func() {
			if err := mq.PublishExtClientDNSUpdate(oldExtClient, newclient, oldExtClient.Network); err != nil {
				logger.Log(1, "error pubishing dns update for extcient update", err.Error())
			}
		}()
	}
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

	err = logic.DeleteExtClient(params["network"], params["clientid"])
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to delete extclient [%s],network [%s]: %v", clientid, network, err))
		err = errors.New("Could not delete extclient " + params["clientid"])
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	go func() {
		if err := mq.PublishDeletedClientPeerUpdate(&extclient); err != nil {
			logger.Log(1, "error setting ext peers on "+ingressnode.ID.String()+": "+err.Error())
		}
		if err = mq.PublishDeleteExtClientDNS(&extclient); err != nil {
			logger.Log(1, "error publishing dns update for extclient deletion", err.Error())
		}
	}()

	logger.Log(0, r.Header.Get("user"),
		"Deleted extclient client", params["clientid"], "from network", params["network"])
	logic.ReturnSuccessResponse(w, r, params["clientid"]+" deleted.")
}

// validateCustomExtClient	Validates the extclient object
func validateCustomExtClient(customExtClient *models.CustomExtClient, checkID bool) error {
	//validate clientid
	if customExtClient.ClientID != "" {
		if err := isValid(customExtClient.ClientID, checkID); err != nil {
			return fmt.Errorf("client validatation: %v", err)
		}
	}
	//extclient.ClientID = customExtClient.ClientID
	if len(customExtClient.PublicKey) > 0 {
		if _, err := wgtypes.ParseKey(customExtClient.PublicKey); err != nil {
			return errInvalidExtClientPubKey
		}
		//extclient.PublicKey = customExtClient.PublicKey
	}
	//validate extra ips
	if len(customExtClient.ExtraAllowedIPs) > 0 {
		for _, ip := range customExtClient.ExtraAllowedIPs {
			if _, _, err := net.ParseCIDR(ip); err != nil {
				return errInvalidExtClientExtraIP
			}
		}
		//extclient.ExtraAllowedIPs = customExtClient.ExtraAllowedIPs
	}
	//validate DNS
	if customExtClient.DNS != "" {
		if ip := net.ParseIP(customExtClient.DNS); ip == nil {
			return errInvalidExtClientDNS
		}
		//extclient.DNS = customExtClient.DNS
	}
	return nil
}

// isValid	Checks if the clientid is valid
func isValid(clientid string, checkID bool) error {
	if !validName(clientid) {
		return errInvalidExtClientID
	}
	if checkID {
		extclients, err := logic.GetAllExtClients()
		if err != nil {
			return fmt.Errorf("extclients isValid: %v", err)
		}
		for _, extclient := range extclients {
			if clientid == extclient.ClientID {
				return errDuplicateExtClientName
			}
		}
	}
	return nil
}
