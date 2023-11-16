package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/slog"
)

func hostHandlers(r *mux.Router) {
	r.HandleFunc("/api/hosts", logic.SecurityCheck(true, http.HandlerFunc(getHosts))).Methods(http.MethodGet)
	r.HandleFunc("/api/hosts/keys", logic.SecurityCheck(true, http.HandlerFunc(updateAllKeys))).Methods(http.MethodPut)
	r.HandleFunc("/api/hosts/{hostid}/keys", logic.SecurityCheck(true, http.HandlerFunc(updateKeys))).Methods(http.MethodPut)
	r.HandleFunc("/api/hosts/{hostid}/sync", logic.SecurityCheck(true, http.HandlerFunc(syncHost))).Methods(http.MethodPost)
	r.HandleFunc("/api/hosts/{hostid}", logic.SecurityCheck(true, http.HandlerFunc(updateHost))).Methods(http.MethodPut)
	r.HandleFunc("/api/hosts/{hostid}", Authorize(true, false, "all", http.HandlerFunc(deleteHost))).Methods(http.MethodDelete)
	r.HandleFunc("/api/hosts/{hostid}/upgrade", logic.SecurityCheck(true, http.HandlerFunc(upgradeHost))).Methods(http.MethodPut)
	r.HandleFunc("/api/hosts/{hostid}/networks/{network}", logic.SecurityCheck(true, http.HandlerFunc(addHostToNetwork))).Methods(http.MethodPost)
	r.HandleFunc("/api/hosts/{hostid}/networks/{network}", logic.SecurityCheck(true, http.HandlerFunc(deleteHostFromNetwork))).Methods(http.MethodDelete)
	r.HandleFunc("/api/hosts/adm/authenticate", authenticateHost).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/host", Authorize(true, false, "host", http.HandlerFunc(pull))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/host/{hostid}/signalpeer", Authorize(true, false, "host", http.HandlerFunc(signalPeer))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/auth-register/host", socketHandler)
}

// upgrade host is a handler to send upgrade message to a host
func upgradeHost(w http.ResponseWriter, r *http.Request) {
	host, err := logic.GetHost(mux.Vars(r)["hostid"])
	if err != nil {
		slog.Error("failed to find host", "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "notfound"))
		return
	}
	if err := mq.HostUpdate(&models.HostUpdate{Action: models.Upgrade, Host: *host}); err != nil {
		slog.Error("failed to upgrade host", "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.ReturnSuccessResponse(w, r, "passed message to upgrade host")
}

// swagger:route GET /api/hosts hosts getHosts
//
// Lists all hosts.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: apiHostResponse
func getHosts(w http.ResponseWriter, r *http.Request) {
	currentHosts, err := logic.GetAllHosts()
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to fetch hosts: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	apiHosts := logic.GetAllHostsAPI(currentHosts[:])
	logger.Log(2, r.Header.Get("user"), "fetched all hosts")
	logic.SortApiHosts(apiHosts[:])
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiHosts)
}

// swagger:route GET /api/v1/host hosts pullHost
//
// Used by clients for "pull" command
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: hostPull
func pull(w http.ResponseWriter, r *http.Request) {

	hostID := r.Header.Get(hostIDHeader) // return JSON/API formatted keys
	if len(hostID) == 0 {
		logger.Log(0, "no host authorized to pull")
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("no host authorized to pull"), "internal"))
		return
	}
	host, err := logic.GetHost(hostID)
	if err != nil {
		logger.Log(0, "no host found during pull", hostID)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	allNodes, err := logic.GetAllNodes()
	if err != nil {
		logger.Log(0, "failed to get nodes: ", hostID)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	hPU, err := logic.GetPeerUpdateForHost("", host, allNodes, nil, nil)
	if err != nil {
		logger.Log(0, "could not pull peers for host", hostID, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	serverConf := servercfg.GetServerInfo()
	if servercfg.GetBrokerType() == servercfg.EmqxBrokerType {
		serverConf.MQUserName = hostID
	}
	key, keyErr := logic.RetrievePublicTrafficKey()
	if keyErr != nil {
		logger.Log(0, "error retrieving key:", keyErr.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	serverConf.TrafficKey = key
	response := models.HostPull{
		Host:            *host,
		Nodes:           logic.GetHostNodes(host),
		ServerConfig:    serverConf,
		Peers:           hPU.Peers,
		PeerIDs:         hPU.PeerIDs,
		HostNetworkInfo: hPU.HostNetworkInfo,
	}

	logger.Log(1, hostID, "completed a pull")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&response)
}

// swagger:route PUT /api/hosts/{hostid} hosts updateHost
//
// Updates a Netclient host on Netmaker server.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: apiHostResponse
func updateHost(w http.ResponseWriter, r *http.Request) {
	var newHostData models.ApiHost
	err := json.NewDecoder(r.Body).Decode(&newHostData)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	// confirm host exists
	currHost, err := logic.GetHost(newHostData.ID)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	newHost := newHostData.ConvertAPIHostToNMHost(currHost)

	logic.UpdateHost(newHost, currHost) // update the in memory struct values
	if err = logic.UpsertHost(newHost); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	// publish host update through MQ
	if err := mq.HostUpdate(&models.HostUpdate{
		Action: models.UpdateHost,
		Host:   *newHost,
	}); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to send host update: ", currHost.ID.String(), err.Error())
	}
	go func() {
		if err := mq.PublishPeerUpdate(); err != nil {
			logger.Log(0, "fail to publish peer update: ", err.Error())
		}
		if newHost.Name != currHost.Name {
			networks := logic.GetHostNetworks(currHost.ID.String())
			if err := mq.PublishHostDNSUpdate(currHost, newHost, networks); err != nil {
				var dnsError *models.DNSError
				if errors.Is(err, dnsError) {
					for _, message := range err.(models.DNSError).ErrorStrings {
						logger.Log(0, message)
					}
				} else {
					logger.Log(0, err.Error())
				}
			}
		}
	}()

	apiHostData := newHost.ConvertNMHostToAPI()
	logger.Log(2, r.Header.Get("user"), "updated host", newHost.ID.String())
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiHostData)
}

// swagger:route DELETE /api/hosts/{hostid} hosts deleteHost
//
// Deletes a Netclient host from Netmaker server.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: apiHostResponse
func deleteHost(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	hostid := params["hostid"]
	forceDelete := r.URL.Query().Get("force") == "true"

	// confirm host exists
	currHost, err := logic.GetHost(hostid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to delete a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	for _, nodeID := range currHost.Nodes {
		node, err := logic.GetNodeByID(nodeID)
		if err != nil {
			slog.Error("failed to get node", "nodeid", nodeID, "error", err)
			continue
		}
		var gwClients []models.ExtClient
		if node.IsIngressGateway {
			gwClients = logic.GetGwExtclients(node.ID.String(), node.Network)
		}
		go mq.PublishMqUpdatesForDeletedNode(node, false, gwClients)

	}
	if err = logic.RemoveHost(currHost, forceDelete); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to delete a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if err = mq.HostUpdate(&models.HostUpdate{
		Action: models.DeleteHost,
		Host:   *currHost,
	}); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to send delete host update: ", currHost.ID.String(), err.Error())
	}

	apiHostData := currHost.ConvertNMHostToAPI()
	logger.Log(2, r.Header.Get("user"), "removed host", currHost.Name)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiHostData)
}

// swagger:route POST /api/hosts/{hostid}/networks/{network} hosts addHostToNetwork
//
// Given a network, a host is added to the network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//			Responses:
//				200: okResponse
func addHostToNetwork(w http.ResponseWriter, r *http.Request) {

	var params = mux.Vars(r)
	hostid := params["hostid"]
	network := params["network"]
	if hostid == "" || network == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("hostid or network cannot be empty"), "badrequest"))
		return
	}
	// confirm host exists
	currHost, err := logic.GetHost(hostid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to find host:", hostid, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	newNode, err := logic.UpdateHostNetwork(currHost, network, true)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to add host to network:", hostid, network, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(1, "added new node", newNode.ID.String(), "to host", currHost.Name)
	go func() {
		mq.HostUpdate(&models.HostUpdate{
			Action: models.JoinHostToNetwork,
			Host:   *currHost,
			Node:   *newNode,
		})
		mq.PublishPeerUpdate()
		mq.HandleNewNodeDNS(currHost, newNode)
	}()
	logger.Log(2, r.Header.Get("user"), fmt.Sprintf("added host %s to network %s", currHost.Name, network))
	w.WriteHeader(http.StatusOK)
}

// swagger:route DELETE /api/hosts/{hostid}/networks/{network} hosts deleteHostFromNetwork
//
// Given a network, a host is removed from the network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: okResponse
func deleteHostFromNetwork(w http.ResponseWriter, r *http.Request) {

	var params = mux.Vars(r)
	hostid := params["hostid"]
	network := params["network"]
	forceDelete := r.URL.Query().Get("force") == "true"
	if hostid == "" || network == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("hostid or network cannot be empty"), "badrequest"))
		return
	}
	// confirm host exists
	currHost, err := logic.GetHost(hostid)
	if err != nil {
		if database.IsEmptyRecord(err) {
			// check if there is any daemon nodes that needs to be deleted
			node, err := logic.GetNodeByHostRef(hostid, network)
			if err != nil {
				slog.Error("couldn't get node for host", "hostid", hostid, "network", network, "error", err)
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
				return
			}
			if err = logic.DeleteNodeByID(&node); err != nil {
				slog.Error("failed to force delete daemon node",
					"nodeid", node.ID.String(), "hostid", hostid, "network", network, "error", err)
				logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to force delete daemon node: "+err.Error()), "internal"))
				return
			}
			logic.ReturnSuccessResponse(w, r, "force deleted daemon node successfully")
			return
		}

		logger.Log(0, r.Header.Get("user"), "failed to find host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	node, err := logic.UpdateHostNetwork(currHost, network, false)
	if err != nil {
		if node == nil && forceDelete {
			// force cleanup the node
			node, err := logic.GetNodeByHostRef(hostid, network)
			if err != nil {
				slog.Error("couldn't get node for host", "hostid", hostid, "network", network, "error", err)
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
				return
			}
			if err = logic.DeleteNodeByID(&node); err != nil {
				slog.Error("failed to force delete daemon node",
					"nodeid", node.ID.String(), "hostid", hostid, "network", network, "error", err)
				logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to force delete daemon node: "+err.Error()), "internal"))
				return
			}
			logic.ReturnSuccessResponse(w, r, "force deleted daemon node successfully")
			return
		}
		logger.Log(0, r.Header.Get("user"), "failed to remove host from network:", hostid, network, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	var gwClients []models.ExtClient
	if node.IsIngressGateway {
		gwClients = logic.GetGwExtclients(node.ID.String(), node.Network)
	}
	logger.Log(1, "deleting node", node.ID.String(), "from host", currHost.Name)
	if err := logic.DeleteNode(node, forceDelete); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to delete node"), "internal"))
		return
	}
	go mq.PublishMqUpdatesForDeletedNode(*node, true, gwClients)
	logger.Log(2, r.Header.Get("user"), fmt.Sprintf("removed host %s from network %s", currHost.Name, network))
	w.WriteHeader(http.StatusOK)
}

// swagger:route POST /api/hosts/adm/authenticate authenticate authenticateHost
//
// Host based authentication for making further API calls.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: successResponse
func authenticateHost(response http.ResponseWriter, request *http.Request) {
	var authRequest models.AuthParams
	var errorResponse = models.ErrorResponse{
		Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
	}

	decoder := json.NewDecoder(request.Body)
	decoderErr := decoder.Decode(&authRequest)
	defer request.Body.Close()

	if decoderErr != nil {
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = decoderErr.Error()
		logger.Log(0, request.Header.Get("user"), "error decoding request body: ",
			decoderErr.Error())
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}
	errorResponse.Code = http.StatusBadRequest
	if authRequest.ID == "" {
		errorResponse.Message = "W1R3: ID can't be empty"
		logger.Log(0, request.Header.Get("user"), errorResponse.Message)
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	} else if authRequest.Password == "" {
		errorResponse.Message = "W1R3: Password can't be empty"
		logger.Log(0, request.Header.Get("user"), errorResponse.Message)
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}
	host, err := logic.GetHost(authRequest.ID)
	if err != nil {
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logger.Log(0, request.Header.Get("user"),
			"error retrieving host: ", err.Error())
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}
	err = bcrypt.CompareHashAndPassword([]byte(host.HostPass), []byte(authRequest.Password))
	if err != nil {
		errorResponse.Code = http.StatusUnauthorized
		errorResponse.Message = "unauthorized"
		logger.Log(0, request.Header.Get("user"),
			"error validating user password: ", err.Error())
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}

	tokenString, err := logic.CreateJWT(authRequest.ID, authRequest.MacAddress, "")
	if tokenString == "" {
		errorResponse.Code = http.StatusUnauthorized
		errorResponse.Message = "unauthorized"
		logger.Log(0, request.Header.Get("user"),
			fmt.Sprintf("%s: %v", errorResponse.Message, err))
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}

	var successResponse = models.SuccessResponse{
		Code:    http.StatusOK,
		Message: "W1R3: Host " + authRequest.ID + " Authorized",
		Response: models.SuccessfulLoginResponse{
			AuthToken: tokenString,
			ID:        authRequest.ID,
		},
	}
	successJSONResponse, jsonError := json.Marshal(successResponse)

	if jsonError != nil {
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logger.Log(0, request.Header.Get("user"),
			"error marshalling resp: ", err.Error())
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}
	response.WriteHeader(http.StatusOK)
	response.Header().Set("Content-Type", "application/json")
	response.Write(successJSONResponse)
}

// swagger:route POST /api/hosts/{hostid}/signalpeer hosts signalPeer
//
// send signal to peer.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: signal
func signalPeer(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	hostid := params["hostid"]
	// confirm host exists
	_, err := logic.GetHost(hostid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to get host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	var signal models.Signal
	w.Header().Set("Content-Type", "application/json")
	err = json.NewDecoder(r.Body).Decode(&signal)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if signal.ToHostPubKey == "" || signal.TurnRelayEndpoint == "" {
		msg := "insufficient data to signal peer"
		logger.Log(0, r.Header.Get("user"), msg)
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New(msg), "badrequest"))
		return
	}
	hosts, err := logic.GetAllHosts()
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	// push the signal to host through mq
	found := false
	for _, hostI := range hosts {
		if hostI.PublicKey.String() == signal.ToHostPubKey {
			// found host publish message and break
			found = true
			err = mq.HostUpdate(&models.HostUpdate{
				Action: models.SignalHost,
				Host:   hostI,
				Signal: signal,
			})
			if err != nil {
				logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("failed to publish signal to peer: "+err.Error()), "badrequest"))
				return
			}
			break
		}
	}
	if !found {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("failed to signal, peer not found"), "badrequest"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(signal)
}

// swagger:route POST /api/hosts/keys hosts updateAllKeys
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
func updateAllKeys(w http.ResponseWriter, r *http.Request) {
	var errorResponse = models.ErrorResponse{}
	w.Header().Set("Content-Type", "application/json")
	hosts, err := logic.GetAllHosts()
	if err != nil {
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logger.Log(0, r.Header.Get("user"),
			"error retrieving hosts ", err.Error())
		logic.ReturnErrorResponse(w, r, errorResponse)
		return
	}
	go func() {
		hostUpdate := models.HostUpdate{}
		hostUpdate.Action = models.UpdateKeys
		for _, host := range hosts {
			hostUpdate.Host = host
			logger.Log(2, "updating host", host.ID.String(), " for a key update")
			if err = mq.HostUpdate(&hostUpdate); err != nil {
				logger.Log(0, "failed to send update to node during a network wide key update", host.ID.String(), err.Error())
			}
		}
	}()
	logger.Log(2, r.Header.Get("user"), "updated keys for all hosts")
	w.WriteHeader(http.StatusOK)
}

// swagger:route POST /api/hosts/{hostid}keys hosts updateKeys
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
func updateKeys(w http.ResponseWriter, r *http.Request) {
	var errorResponse = models.ErrorResponse{}
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	hostid := params["hostid"]
	host, err := logic.GetHost(hostid)
	if err != nil {
		logger.Log(0, "failed to retrieve host", hostid, err.Error())
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logger.Log(0, r.Header.Get("user"),
			"error retrieving hosts ", err.Error())
		logic.ReturnErrorResponse(w, r, errorResponse)
		return
	}
	go func() {
		hostUpdate := models.HostUpdate{
			Action: models.UpdateKeys,
			Host:   *host,
		}
		if err = mq.HostUpdate(&hostUpdate); err != nil {
			logger.Log(0, "failed to send host key update", host.ID.String(), err.Error())
		}
	}()
	logger.Log(2, r.Header.Get("user"), "updated key on host", host.Name)
	w.WriteHeader(http.StatusOK)
}

// swagger:route POST /api/hosts/{hostid}/sync hosts synchost
//
// Requests a host to pull.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: networkBodyResponse
func syncHost(w http.ResponseWriter, r *http.Request) {
	hostId := mux.Vars(r)["hostid"]

	var errorResponse = models.ErrorResponse{}
	w.Header().Set("Content-Type", "application/json")

	host, err := logic.GetHost(hostId)
	if err != nil {
		slog.Error("failed to retrieve host", "user", r.Header.Get("user"), "error", err)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logic.ReturnErrorResponse(w, r, errorResponse)
		return
	}

	go func() {
		hostUpdate := models.HostUpdate{
			Action: models.RequestPull,
			Host:   *host,
		}
		if err = mq.HostUpdate(&hostUpdate); err != nil {
			slog.Error("failed to send host pull request", "host", host.ID.String(), "error", err)
		}
	}()

	slog.Info("requested host pull", "user", r.Header.Get("user"), "host", host.ID)
	w.WriteHeader(http.StatusOK)
}
