package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/servercfg"

	"github.com/gravitl/netmaker/models"

	"github.com/gravitl/netmaker/mq"
	"github.com/skip2/go-qrcode"
	"golang.org/x/exp/slices"
	"golang.org/x/exp/slog"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func extClientHandlers(r *mux.Router) {

	r.HandleFunc("/api/extclients", logic.SecurityCheck(true, http.HandlerFunc(getAllExtClients))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/extclients/{network}", logic.SecurityCheck(true, http.HandlerFunc(getNetworkExtClients))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/extclients/{network}/{clientid}", logic.SecurityCheck(false, http.HandlerFunc(getExtClient))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/extclients/{network}/{clientid}/{type}", logic.SecurityCheck(false, http.HandlerFunc(getExtClientConf))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/extclients/{network}/{clientid}", logic.SecurityCheck(false, http.HandlerFunc(updateExtClient))).
		Methods(http.MethodPut)
	r.HandleFunc("/api/extclients/{network}/{clientid}", logic.SecurityCheck(false, http.HandlerFunc(deleteExtClient))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/extclients/{network}/{nodeid}", logic.SecurityCheck(false, checkFreeTierLimits(limitChoiceMachines, http.HandlerFunc(createExtClient)))).
		Methods(http.MethodPost)
}

func checkIngressExists(nodeID string) bool {
	node, err := logic.GetNodeByID(nodeID)
	if err != nil {
		return false
	}
	return node.IsIngressGateway
}

// @Summary     Get all extclients associated with network.
// @Router      /api/extclients/{network} [get]
// @Tags        ExtClients
// @Security    oauth2
// @Success     200 {object} models.ExtClient
// @Failure     500 {object} models.ErrorResponse
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

// @Summary     A separate function to get all extclients, not just extclients for a particular network.
// @Router      /api/extclients [get]
// @Tags        ExtClients
// @Security    oauth2
// @Success     200 {object} models.ExtClient
// @Failure     500 {object} models.ErrorResponse
// Not quite sure if this is necessary. Probably necessary based on front end but may
// want to review after iteration 1 if it's being used or not
func getAllExtClients(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	clients, err := logic.GetAllExtClients()
	if err != nil && !database.IsEmptyRecord(err) {
		logger.Log(0, "failed to get all extclients: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	//Return all the extclients in JSON format
	logic.SortExtClient(clients[:])
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(clients)
}

// @Summary     Get an individual extclient.
// @Router      /api/extclients/{network}/{clientid} [get]
// @Tags        ExtClients
// @Security    oauth2
// @Success     200 {object} models.ExtClient
// @Failure     500 {object} models.ErrorResponse
// @Failure     403 {object} models.ErrorResponse
func getExtClient(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	clientid := params["clientid"]
	network := params["network"]
	client, err := logic.GetExtClient(clientid, network)
	if err != nil {
		logger.Log(
			0,
			r.Header.Get("user"),
			fmt.Sprintf("failed to get extclient for [%s] on network [%s]: %v",
				clientid, network, err),
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if !logic.IsUserAllowedAccessToExtClient(r.Header.Get("user"), client) {
		// check if user has access to extclient
		slog.Error("failed to get extclient", "network", network, "clientID",
			clientid, "error", errors.New("access is denied"))
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("access is denied"), "forbidden"),
		)
		return

	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(client)
}

// @Summary     Get an individual extclient.
// @Router      /api/extclients/{network}/{clientid}/{type} [get]
// @Tags        ExtClients
// @Security    oauth2
// @Success     200 {object} models.ExtClient
// @Failure     500 {object} models.ErrorResponse
// @Failure     403 {object} models.ErrorResponse
func getExtClientConf(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	clientid := params["clientid"]
	networkid := params["network"]
	client, err := logic.GetExtClient(clientid, networkid)
	if err != nil {
		logger.Log(
			0,
			r.Header.Get("user"),
			fmt.Sprintf("failed to get extclient for [%s] on network [%s]: %v",
				clientid, networkid, err),
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if !logic.IsUserAllowedAccessToExtClient(r.Header.Get("user"), client) {
		slog.Error("failed to get extclient", "network", networkid, "clientID",
			clientid, "error", errors.New("access is denied"))
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("access is denied"), "forbidden"),
		)
		return
	}

	gwnode, err := logic.GetNodeByID(client.IngressGatewayID)
	if err != nil {
		logger.Log(
			0,
			r.Header.Get("user"),
			fmt.Sprintf(
				"failed to get ingress gateway node [%s] info: %v",
				client.IngressGatewayID,
				err,
			),
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	host, err := logic.GetHost(gwnode.HostID.String())
	if err != nil {
		logger.Log(
			0,
			r.Header.Get("user"),
			fmt.Sprintf(
				"failed to get host for ingress gateway node [%s] info: %v",
				client.IngressGatewayID,
				err,
			),
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	network, err := logic.GetParentNetwork(client.Network)
	if err != nil {
		logger.Log(
			1,
			r.Header.Get("user"),
			"Could not retrieve Ingress Gateway Network",
			client.Network,
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	preferredIp := strings.TrimSpace(r.URL.Query().Get("preferredip"))
	if preferredIp != "" {
		allowedPreferredIps := []string{}
		for i := range gwnode.AdditionalRagIps {
			allowedPreferredIps = append(allowedPreferredIps, gwnode.AdditionalRagIps[i].String())
		}
		allowedPreferredIps = append(allowedPreferredIps, host.EndpointIP.String())
		allowedPreferredIps = append(allowedPreferredIps, host.EndpointIPv6.String())
		if !slices.Contains(allowedPreferredIps, preferredIp) {
			slog.Warn(
				"preferred endpoint ip is not associated with the RAG. proceeding with preferred ip",
				"preferred ip",
				preferredIp,
			)
			logic.ReturnErrorResponse(
				w,
				r,
				logic.FormatError(
					errors.New("preferred endpoint ip is not associated with the RAG"),
					"badrequest",
				),
			)
			return
		}
		if net.ParseIP(preferredIp).To4() == nil {
			preferredIp = fmt.Sprintf("[%s]", preferredIp)
		}
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
	if preferredIp == "" {
		if host.EndpointIP.To4() == nil {
			gwendpoint = fmt.Sprintf("[%s]:%d", host.EndpointIPv6.String(), host.ListenPort)
		} else {
			gwendpoint = fmt.Sprintf("%s:%d", host.EndpointIP.String(), host.ListenPort)
		}
	} else {
		gwendpoint = fmt.Sprintf("%s:%d", preferredIp, host.ListenPort)
	}

	var newAllowedIPs string
	if logic.IsInternetGw(gwnode) || gwnode.InternetGwID != "" {
		egressrange := "0.0.0.0/0"
		if gwnode.Address6.IP != nil && client.Address6 != "" {
			egressrange += "," + "::/0"
		}
		newAllowedIPs = egressrange
	} else {
		newAllowedIPs = network.AddressRange
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

	postUp := strings.Builder{}
	if client.PostUp != "" && params["type"] != "qr" {
		for _, loc := range strings.Split(client.PostUp, "\n") {
			postUp.WriteString(fmt.Sprintf("PostUp = %s\n", loc))
		}
	}

	postDown := strings.Builder{}
	if client.PostDown != "" && params["type"] != "qr" {
		for _, loc := range strings.Split(client.PostDown, "\n") {
			postDown.WriteString(fmt.Sprintf("PostDown = %s\n", loc))
		}
	}

	config := fmt.Sprintf(`[Interface]
Address = %s
PrivateKey = %s
MTU = %d
%s
%s
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
		postUp.String(),
		postDown.String(),
		host.PublicKey,
		newAllowedIPs,
		gwendpoint,
		keepalive,
	)

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

// @Summary     Create an individual extclient. Must have valid key and be unique.
// @Router      /api/extclients/{network}/{nodeid} [post]
// @Tags        ExtClients
// @Security    oauth2
// @Success     200 {string} string "OK"
// @Failure     500 {object} models.ErrorResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     403 {object} models.ErrorResponse
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

	var gateway models.EgressGatewayRequest
	gateway.NetID = params["network"]
	gateway.Ranges = customExtClient.ExtraAllowedIPs
	err := logic.ValidateEgressRange(gateway)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error validating egress range: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	node, err := logic.GetNodeByID(nodeid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get ingress gateway node [%s] info: %v", nodeid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	var userName string
	if r.Header.Get("ismaster") == "yes" {
		userName = logic.MasterUser
	} else {
		caller, err := logic.GetUser(r.Header.Get("user"))
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		userName = caller.UserName
		if _, ok := caller.RemoteGwIDs[nodeid]; (!caller.IsAdmin && !caller.IsSuperAdmin) && !ok {
			err = errors.New("permission denied")
			slog.Error("failed to create extclient", "error", err)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "forbidden"))
			return
		}
		// check if user has a config already for remote access client
		extclients, err := logic.GetNetworkExtClients(node.Network)
		if err != nil {
			slog.Error("failed to get extclients", "error", err)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		for _, extclient := range extclients {
			if extclient.RemoteAccessClientID != "" &&
				extclient.RemoteAccessClientID == customExtClient.RemoteAccessClientID && extclient.OwnerID == caller.UserName && nodeid == extclient.IngressGatewayID {
				// extclient on the gw already exists for the remote access client
				err = errors.New("remote client config already exists on the gateway")
				slog.Error("failed to create extclient", "user", userName, "error", err)
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
				return
			}
		}
	}

	extclient := logic.UpdateExtClient(&models.ExtClient{}, &customExtClient)
	extclient.OwnerID = userName
	extclient.RemoteAccessClientID = customExtClient.RemoteAccessClientID
	extclient.IngressGatewayID = nodeid

	// set extclient dns to ingressdns if extclient dns is not explicitly set
	if (extclient.DNS == "") && (node.IngressDNS != "") {
		extclient.DNS = node.IngressDNS
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

	if err = logic.CreateExtClient(&extclient); err != nil {
		slog.Error(
			"failed to create extclient",
			"user",
			r.Header.Get("user"),
			"network",
			node.Network,
			"error",
			err,
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	slog.Info(
		"created extclient",
		"user",
		r.Header.Get("user"),
		"network",
		node.Network,
		"clientid",
		extclient.ClientID,
	)
	w.WriteHeader(http.StatusOK)
	go func() {
		if err := logic.SetClientDefaultACLs(&extclient); err != nil {
			slog.Error(
				"failed to set default acls for extclient",
				"user",
				r.Header.Get("user"),
				"network",
				node.Network,
				"error",
				err,
			)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		if err := mq.PublishPeerUpdate(false); err != nil {
			logger.Log(1, "error setting ext peers on "+nodeid+": "+err.Error())
		}
		if servercfg.IsDNSMode() {
			logic.SetDNS()
		}
	}()
}

// @Summary     Update an individual extclient.
// @Router      /api/extclients/{network}/{clientid} [put]
// @Tags        ExtClients
// @Security    oauth2
// @Success     200 {object} models.ExtClient
// @Failure     500 {object} models.ErrorResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     403 {object} models.ErrorResponse
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
	network := params["network"]
	oldExtClient, err := logic.GetExtClientByName(clientid)
	if err != nil {
		slog.Error(
			"failed to retrieve extclient",
			"user",
			r.Header.Get("user"),
			"id",
			clientid,
			"error",
			err,
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if !logic.IsUserAllowedAccessToExtClient(r.Header.Get("user"), oldExtClient) {
		// check if user has access to extclient
		slog.Error("failed to get extclient", "network", network, "clientID",
			clientid, "error", errors.New("access is denied"))
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("access is denied"), "forbidden"),
		)
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

	var gateway models.EgressGatewayRequest
	gateway.NetID = params["network"]
	gateway.Ranges = update.ExtraAllowedIPs
	err = logic.ValidateEgressRange(gateway)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error validating egress range: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	var changedID = update.ClientID != oldExtClient.ClientID

	if !reflect.DeepEqual(update.DeniedACLs, oldExtClient.DeniedACLs) {
		sendPeerUpdate = true
		logic.SetClientACLs(&oldExtClient, update.DeniedACLs)
	}
	if !logic.IsSlicesEqual(update.ExtraAllowedIPs, oldExtClient.ExtraAllowedIPs) {
		sendPeerUpdate = true
	}

	if update.Enabled != oldExtClient.Enabled {
		sendPeerUpdate = true
	}
	newclient := logic.UpdateExtClient(&oldExtClient, &update)
	if err := logic.DeleteExtClient(oldExtClient.Network, oldExtClient.ClientID); err != nil {
		slog.Error(
			"failed to delete ext client",
			"user",
			r.Header.Get("user"),
			"id",
			oldExtClient.ClientID,
			"network",
			oldExtClient.Network,
			"error",
			err,
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if err := logic.SaveExtClient(&newclient); err != nil {
		slog.Error(
			"failed to save ext client",
			"user",
			r.Header.Get("user"),
			"id",
			newclient.ClientID,
			"network",
			newclient.Network,
			"error",
			err,
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(0, r.Header.Get("user"), "updated ext client", update.ClientID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newclient)

	go func() {
		if changedID && servercfg.IsDNSMode() {
			logic.SetDNS()
		}
		if sendPeerUpdate { // need to send a peer update to the ingress node as enablement of one of it's clients has changed
			ingressNode, err := logic.GetNodeByID(newclient.IngressGatewayID)
			if err == nil {
				if err = mq.PublishPeerUpdate(false); err != nil {
					logger.Log(
						1,
						"error setting ext peers on",
						ingressNode.ID.String(),
						":",
						err.Error(),
					)
				}
			}
			if !update.Enabled {
				ingressHost, err := logic.GetHost(ingressNode.HostID.String())
				if err != nil {
					slog.Error(
						"Failed to get ingress host",
						"node",
						ingressNode.ID.String(),
						"error",
						err,
					)
					return
				}
				nodes, err := logic.GetAllNodes()
				if err != nil {
					slog.Error("Failed to get nodes", "error", err)
					return
				}
				go mq.PublishSingleHostPeerUpdate(
					ingressHost,
					nodes,
					nil,
					[]models.ExtClient{oldExtClient},
					false,
				)
			}
		}

	}()

}

// @Summary     Delete an individual extclient.
// @Router      /api/extclients/{network}/{clientid} [delete]
// @Tags        ExtClients
// @Security    oauth2
// @Success     200
// @Failure     500 {object} models.ErrorResponse
// @Failure     403 {object} models.ErrorResponse
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
			fmt.Sprintf("failed to get extclient [%s],network [%s]: %v", clientid, network, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if !logic.IsUserAllowedAccessToExtClient(r.Header.Get("user"), extclient) {
		slog.Error("user not allowed to delete", "network", network, "clientID",
			clientid, "error", errors.New("access is denied"))
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("access is denied"), "forbidden"),
		)
		return
	}
	ingressnode, err := logic.GetNodeByID(extclient.IngressGatewayID)
	if err != nil {
		logger.Log(
			0,
			r.Header.Get("user"),
			fmt.Sprintf(
				"failed to get ingress gateway node [%s] info: %v",
				extclient.IngressGatewayID,
				err,
			),
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	err = logic.DeleteExtClientAndCleanup(extclient)
	if err != nil {
		slog.Error("deleteExtClient: ", "Error", err.Error())
		err = errors.New("Could not delete extclient " + params["clientid"])
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	go func() {
		if err := mq.PublishDeletedClientPeerUpdate(&extclient); err != nil {
			slog.Error("error setting ext peers on " + ingressnode.ID.String() + ": " + err.Error())
		}
		if servercfg.IsDNSMode() {
			logic.SetDNS()
		}
	}()

	logger.Log(0, r.Header.Get("user"),
		"Deleted extclient client", params["clientid"], "from network", params["network"])
	logic.ReturnSuccessResponse(w, r, params["clientid"]+" deleted.")
}

// validateCustomExtClient	Validates the extclient object
func validateCustomExtClient(customExtClient *models.CustomExtClient, checkID bool) error {
	v := validator.New()
	err := v.Struct(customExtClient)
	if err != nil {
		return err
	}
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
