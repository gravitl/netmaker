package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gravitl/netmaker/converters"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/gorm"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/slog"
)

func hostHandlers(r *mux.Router) {
	r.HandleFunc("/api/hosts", logic.SecurityCheck(true, http.HandlerFunc(getHosts))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/hosts/keys", logic.SecurityCheck(true, http.HandlerFunc(updateAllKeys))).
		Methods(http.MethodPut)
	r.HandleFunc("/api/hosts/sync", logic.SecurityCheck(true, http.HandlerFunc(syncHosts))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/hosts/upgrade", logic.SecurityCheck(true, http.HandlerFunc(upgradeHosts))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/hosts/{hostid}/keys", logic.SecurityCheck(true, http.HandlerFunc(updateKeys))).
		Methods(http.MethodPut)
	r.HandleFunc("/api/hosts/{hostid}/sync", logic.SecurityCheck(true, http.HandlerFunc(syncHost))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/hosts/{hostid}", logic.SecurityCheck(true, http.HandlerFunc(updateHost))).
		Methods(http.MethodPut)
	r.HandleFunc("/api/hosts/{hostid}", Authorize(true, false, "all", http.HandlerFunc(deleteHost))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/hosts/{hostid}/upgrade", logic.SecurityCheck(true, http.HandlerFunc(upgradeHost))).
		Methods(http.MethodPut)
	r.HandleFunc("/api/hosts/{hostid}/networks/{network}", logic.SecurityCheck(true, http.HandlerFunc(addHostToNetwork))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/hosts/{hostid}/networks/{network}", logic.SecurityCheck(true, http.HandlerFunc(deleteHostFromNetwork))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/hosts/adm/authenticate", authenticateHost).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/host", Authorize(true, false, "host", http.HandlerFunc(pull))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/host/{hostid}/signalpeer", Authorize(true, false, "host", http.HandlerFunc(signalPeer))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/fallback/host/{hostid}", Authorize(true, false, "host", http.HandlerFunc(hostUpdateFallback))).
		Methods(http.MethodPut)
	r.HandleFunc("/api/v1/host/{hostid}/peer_info", Authorize(true, false, "host", http.HandlerFunc(getHostPeerInfo))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/emqx/hosts", logic.SecurityCheck(true, http.HandlerFunc(delEmqxHosts))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/auth-register/host", socketHandler)
}

// @Summary     Requests all the hosts to upgrade their version
// @Router      /api/hosts/upgrade [post]
// @Tags        Hosts
// @Security    oauth
// @Param       force query bool false "Force upgrade"
// @Success     200 {string} string "upgrade all hosts request received"
func upgradeHosts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	action := models.Upgrade

	if r.URL.Query().Get("force") == "true" {
		action = models.ForceUpgrade
	}

	user := r.Header.Get("user")

	go func() {
		slog.Info("requesting all hosts to upgrade", "user", user)

		_hosts, err := (&schema.Host{}).ListAll(r.Context())
		if err != nil {
			slog.Error("failed to retrieve all hosts", "user", user, "error", err)
			return
		}

		hosts := converters.ToModelHosts(_hosts)
		for _, host := range hosts {
			go func(host models.Host) {
				hostUpdate := models.HostUpdate{
					Action: action,
					Host:   host,
				}
				if err = mq.HostUpdate(&hostUpdate); err != nil {
					slog.Error("failed to request host to upgrade", "user", user, "host", host.ID.String(), "error", err)
				} else {
					slog.Info("host upgrade requested", "user", user, "host", host.ID.String())
				}
			}(host)
		}
	}()

	slog.Info("upgrade all hosts request received", "user", user)
	logic.ReturnSuccessResponse(w, r, "upgrade all hosts request received")
}

// @Summary     Upgrade a host
// @Router      /api/hosts/{hostid}/upgrade [put]
// @Tags        Hosts
// @Security    oauth
// @Param       hostid path string true "Host ID"
// @Param       force query bool false "Force upgrade"
// @Success     200 {string} string "passed message to upgrade host"
// @Failure     500 {object} models.ErrorResponse
// upgrade host is a handler to send upgrade message to a host
func upgradeHost(w http.ResponseWriter, r *http.Request) {
	hostID := mux.Vars(r)["hostid"]
	_host := &schema.Host{
		ID: hostID,
	}
	err := _host.Get(r.Context())
	if err != nil {
		slog.Error("failed to find host", "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "notfound"))
		return
	}

	host := converters.ToModelHost(*_host)

	action := models.Upgrade

	if r.URL.Query().Get("force") == "true" {
		action = models.ForceUpgrade
	}

	if err := mq.HostUpdate(&models.HostUpdate{Action: action, Host: host}); err != nil {
		slog.Error("failed to upgrade host", "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.ReturnSuccessResponse(w, r, "passed message to upgrade host")
}

// @Summary     List all hosts
// @Router      /api/hosts [get]
// @Tags        Hosts
// @Security    oauth
// @Success     200 {array} models.ApiHost
// @Failure     500 {object} models.ErrorResponse
func getHosts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_hosts, err := (&schema.Host{}).ListAll(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to fetch hosts: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	hosts := converters.ToAPIHosts(_hosts)
	logger.Log(2, r.Header.Get("user"), "fetched all hosts")
	logic.SortApiHosts(hosts)

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(hosts)
}

// @Summary     Used by clients for "pull" command
// @Router      /api/v1/host [get]
// @Tags        Hosts
// @Security    oauth
// @Success     200 {object} models.HostPull
// @Failure     500 {object} models.ErrorResponse
func pull(w http.ResponseWriter, r *http.Request) {
	hostID := r.Header.Get(hostIDHeader) // return JSON/API formatted keys
	if len(hostID) == 0 {
		logger.Log(0, "no host authorized to pull")
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(fmt.Errorf("no host authorized to pull"), "internal"),
		)
		return
	}

	_host := &schema.Host{
		ID: hostID,
	}
	err := _host.Get(r.Context())
	if err != nil {
		logger.Log(0, "no host found during pull", hostID)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	err = _host.GetNodes(r.Context())
	if err != nil {
		logger.Log(0, "failed to get host's nodes", hostID)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	sendPeerUpdate := false
	for _, _node := range _host.Nodes {
		if _node.FailOverNodeID != nil && r.URL.Query().Get("reset_failovered") == "true" {
			node := converters.ToModelNode(_node)
			_ = logic.ResetFailedOverPeer(&node)
			sendPeerUpdate = true
		}
	}

	if sendPeerUpdate {
		if err := mq.PublishPeerUpdate(false); err != nil {
			logger.Log(0, "fail to publish peer update: ", err.Error())
		}
	}

	host := converters.ToModelHost(*_host)
	hPU, err := logic.GetPeerUpdateForHost("", &host, nil, nil)
	if err != nil {
		logger.Log(0, "could not pull peers for host", hostID, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	serverConf := servercfg.GetServerInfo()
	key, keyErr := logic.RetrievePublicTrafficKey()
	if keyErr != nil {
		logger.Log(0, "error retrieving key:", keyErr.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	serverConf.TrafficKey = key
	response := models.HostPull{
		Host:              converters.ToModelHost(*_host),
		Nodes:             converters.ToModelNodes(_host.Nodes),
		ServerConfig:      serverConf,
		Peers:             hPU.Peers,
		PeerIDs:           hPU.PeerIDs,
		HostNetworkInfo:   hPU.HostNetworkInfo,
		EgressRoutes:      hPU.EgressRoutes,
		FwUpdate:          hPU.FwUpdate,
		ChangeDefaultGw:   hPU.ChangeDefaultGw,
		DefaultGwIp:       hPU.DefaultGwIp,
		IsInternetGw:      hPU.IsInternetGw,
		EndpointDetection: servercfg.IsEndpointDetectionEnabled(),
	}

	logger.Log(1, hostID, "completed a pull")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(&response)
}

// @Summary     Updates a Netclient host on Netmaker server
// @Router      /api/hosts/{hostid} [put]
// @Tags        Hosts
// @Security    oauth
// @Param       hostid path string true "Host ID"
// @Param       body body models.ApiHost true "New host data"
// @Success     200 {object} models.ApiHost
// @Failure     500 {object} models.ErrorResponse
func updateHost(w http.ResponseWriter, r *http.Request) {
	hostID := mux.Vars(r)["hostid"]

	var hostChanges models.ApiHost
	err := json.NewDecoder(r.Body).Decode(&hostChanges)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	_host := &schema.Host{
		ID: hostID,
	}
	err = _host.Get(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	// assign changes to host.
	var nameChanged bool
	if len(hostChanges.Name) != 0 {
		_host.Name = hostChanges.Name
		nameChanged = true
	}

	if len(hostChanges.Version) != 0 {
		_host.Version = hostChanges.Version
	}

	if hostChanges.MTU != 0 {
		_host.MTU = hostChanges.MTU
	}

	if hostChanges.ListenPort != 0 {
		_host.ListenPort = hostChanges.ListenPort
	}

	if hostChanges.PersistentKeepalive != 0 {
		_host.PersistentKeepalive = time.Duration(hostChanges.PersistentKeepalive) * time.Second
	}

	err = _host.Upsert(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	// publish host update through MQ
	if err := mq.HostUpdate(&models.HostUpdate{
		Action: models.UpdateHost,
		Host:   converters.ToModelHost(*_host),
	}); err != nil {
		logger.Log(
			0,
			r.Header.Get("user"),
			"failed to send host update: ",
			hostID,
			err.Error(),
		)
	}
	go func() {
		if err := mq.PublishPeerUpdate(false); err != nil {
			logger.Log(0, "fail to publish peer update: ", err.Error())
		}
		if nameChanged {
			if servercfg.IsDNSMode() {
				logic.SetDNS()
			}
		}
	}()

	apiHost := converters.ToAPIHost(*_host)
	logger.Log(2, r.Header.Get("user"), "updated host", hostID)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(apiHost)
}

// @Summary     Updates a Netclient host on Netmaker server
// @Router      /api/v1/fallback/host/{hostid} [put]
// @Tags        Hosts
// @Security    oauth
// @Param       hostid path string true "Host ID"
// @Param       body body models.HostUpdate true "Host update data"
// @Success     200 {string} string "updated host data"
// @Failure     500 {object} models.ErrorResponse
func hostUpdateFallback(w http.ResponseWriter, r *http.Request) {
	hostID := mux.Vars(r)["hostid"]

	_host := &schema.Host{
		ID: hostID,
	}
	err := _host.Get(r.Context())
	if err != nil {
		slog.Error("error getting host", "id", hostID, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	var sendPeerUpdate bool
	var replacePeers bool
	var hostUpdate models.HostUpdate
	err = json.NewDecoder(r.Body).Decode(&hostUpdate)
	if err != nil {
		slog.Error("failed to update a host:", "user", r.Header.Get("user"), "error", err.Error(), "host", _host.Name)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	slog.Info("received host update", "name", hostUpdate.Host.Name, "id", hostUpdate.Host.ID, "action", hostUpdate.Action)

	host := converters.ToModelHost(*_host)
	switch hostUpdate.Action {
	case models.CheckIn:
		sendPeerUpdate = mq.HandleHostCheckin(&hostUpdate.Host, &host)

	case models.UpdateHost:
		if hostUpdate.Host.PublicKey.String() != _host.PublicKey {
			//remove old peer entry
			replacePeers = true
		}
		sendPeerUpdate = logic.UpdateHostFromClient(&hostUpdate.Host, &host)

		_host := converters.ToSchemaHost(host)
		err = _host.Upsert(db.WithContext(r.Context()))
		if err != nil {
			slog.Error("failed to update host", "id", hostID, "error", err)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}

	case models.UpdateMetrics:
		mq.UpdateMetricsFallBack(hostUpdate.Node.ID.String(), hostUpdate.NewMetrics)
	}

	if sendPeerUpdate {
		err := mq.PublishPeerUpdate(replacePeers)
		if err != nil {
			slog.Error("failed to publish peer update", "error", err)
		}
	}
	logic.ReturnSuccessResponse(w, r, "updated host data")
}

// @Summary     Deletes a Netclient host from Netmaker server
// @Router      /api/hosts/{hostid} [delete]
// @Tags        Hosts
// @Security    oauth
// @Param       hostid path string true "Host ID"
// @Param       force query bool false "Force delete"
// @Success     200 {object} models.ApiHost
// @Failure     500 {object} models.ErrorResponse
func deleteHost(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	hostID := params["hostid"]
	forceDelete := r.URL.Query().Get("force") == "true"

	// confirm host exists
	_host := &schema.Host{
		ID: hostID,
	}
	err := _host.Get(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to delete a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	err = _host.GetNodes(r.Context())
	if err != nil {
		slog.Error("failed to get host nodes", "hostid", hostID, "error", err)
	}

	for _, _node := range _host.Nodes {
		var gwClients []models.ExtClient
		if _node.GatewayNodeConfig != nil {
			gwClients = logic.GetGwExtclients(_node.ID, _node.NetworkID)
		}

		go mq.PublishMqUpdatesForDeletedNode(converters.ToModelNode(_node), false, gwClients)
	}

	if servercfg.GetBrokerType() == servercfg.EmqxBrokerType {
		// delete EMQX credentials for host
		if err := mq.GetEmqxHandler().DeleteEmqxUser(hostID); err != nil {
			slog.Error(
				"failed to remove host credentials from EMQX",
				"id",
				hostID,
				"error",
				err,
			)
		}
	}

	if err = mq.HostUpdate(&models.HostUpdate{
		Action: models.DeleteHost,
		Host:   converters.ToModelHost(*_host),
	}); err != nil {
		logger.Log(
			0,
			r.Header.Get("user"),
			"failed to send delete host update: ",
			hostID,
			err.Error(),
		)
	}

	host := converters.ToModelHost(*_host)
	if err = logic.RemoveHost(&host, forceDelete); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to delete a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	apiHost := converters.ToAPIHost(*_host)
	logger.Log(2, r.Header.Get("user"), "removed host", _host.Name)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(apiHost)
}

// @Summary     To Add Host To Network
// @Router      /api/hosts/{hostid}/networks/{network} [post]
// @Tags        Hosts
// @Security    oauth
// @Param       hostid path string true "Host ID"
// @Param       network path string true "Network name"
// @Success     200 {string} string "OK"
// @Failure     500 {object} models.ErrorResponse
func addHostToNetwork(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	hostID := params["hostid"]
	network := params["network"]
	if hostID == "" || network == "" {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("hostid or network cannot be empty"), "badrequest"),
		)
		return
	}

	_host := &schema.Host{
		ID: hostID,
	}
	err := _host.Get(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to find host:", hostID, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	host := converters.ToModelHost(*_host)
	newNode, err := logic.UpdateHostNetwork(&host, network, true)
	if err != nil {
		logger.Log(
			0,
			r.Header.Get("user"),
			"failed to add host to network:",
			hostID,
			network,
			err.Error(),
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(1, "added new node", newNode.ID.String(), "to host", _host.Name)
	if _host.IsDefault {
		// make  host failover
		logic.CreateFailOver(*newNode)
		// make host remote access gateway
		logic.CreateIngressGateway(network, newNode.ID.String(), models.IngressRequest{})
	}
	go func() {
		mq.HostUpdate(&models.HostUpdate{
			Action: models.JoinHostToNetwork,
			Host:   host,
			Node:   *newNode,
		})
		mq.PublishPeerUpdate(false)
		if servercfg.IsDNSMode() {
			logic.SetDNS()
		}
	}()
	logger.Log(
		2,
		r.Header.Get("user"),
		fmt.Sprintf("added host %s to network %s", host.Name, network),
	)
	w.WriteHeader(http.StatusOK)
}

// @Summary     To Remove Host from Network
// @Router      /api/hosts/{hostid}/networks/{network} [delete]
// @Tags        Hosts
// @Security    oauth
// @Param       hostid path string true "Host ID"
// @Param       network path string true "Network name"
// @Param       force query bool false "Force delete"
// @Success     200 {string} string "OK"
// @Failure     500 {object} models.ErrorResponse
func deleteHostFromNetwork(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	hostID := params["hostid"]
	network := params["network"]
	forceDelete := r.URL.Query().Get("force") == "true"
	if hostID == "" || network == "" {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("hostid or network cannot be empty"), "badrequest"),
		)
		return
	}

	_host := &schema.Host{
		ID: hostID,
	}
	err := _host.Get(r.Context())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// check if there is any daemon nodes that needs to be deleted
			node, err := logic.GetNodeByHostRef(hostID, network)
			if err != nil {
				slog.Error(
					"couldn't get node for host",
					"hostid",
					hostID,
					"network",
					network,
					"error",
					err,
				)
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
				return
			}
			if err = logic.DeleteNodeByID(&node); err != nil {
				slog.Error("failed to force delete daemon node",
					"nodeid", node.ID.String(), "hostid", hostID, "network", network, "error", err)
				logic.ReturnErrorResponse(
					w,
					r,
					logic.FormatError(
						fmt.Errorf("failed to force delete daemon node: %s", err.Error()),
						"internal",
					),
				)
				return
			}
			logic.ReturnSuccessResponse(w, r, "force deleted daemon node successfully")
			return
		}

		logger.Log(0, r.Header.Get("user"), "failed to find host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	host := converters.ToModelHost(*_host)
	node, err := logic.UpdateHostNetwork(&host, network, false)
	if err != nil {
		if node == nil && forceDelete {
			// force cleanup the node
			node, err := logic.GetNodeByHostRef(hostID, network)
			if err != nil {
				slog.Error(
					"couldn't get node for host",
					"hostid",
					hostID,
					"network",
					network,
					"error",
					err,
				)
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
				return
			}
			if err = logic.DeleteNodeByID(&node); err != nil {
				slog.Error("failed to force delete daemon node",
					"nodeid", node.ID.String(), "hostid", hostID, "network", network, "error", err)
				logic.ReturnErrorResponse(
					w,
					r,
					logic.FormatError(
						fmt.Errorf("failed to force delete daemon node: %s", err.Error()),
						"internal",
					),
				)
				return
			}
			logic.ReturnSuccessResponse(w, r, "force deleted daemon node successfully")
			return
		}
		logger.Log(
			0,
			r.Header.Get("user"),
			"failed to remove host from network:",
			hostID,
			network,
			err.Error(),
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	var gwClients []models.ExtClient
	if node.IsIngressGateway {
		gwClients = logic.GetGwExtclients(node.ID.String(), node.Network)
	}
	logger.Log(1, "deleting node", node.ID.String(), "from host", _host.Name)
	if err := logic.DeleteNode(node, forceDelete); err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(fmt.Errorf("failed to delete node"), "internal"),
		)
		return
	}
	go func() {
		mq.PublishMqUpdatesForDeletedNode(*node, true, gwClients)
		if servercfg.IsDNSMode() {
			logic.SetDNS()
		}
	}()
	logger.Log(
		2,
		r.Header.Get("user"),
		fmt.Sprintf("removed host %s from network %s", _host.Name, network),
	)
	w.WriteHeader(http.StatusOK)
}

// @Summary     To Fetch Auth Token for a Host
// @Router      /api/hosts/adm/authenticate [post]
// @Tags        Auth
// @Accept      json
// @Param       body body models.AuthParams true "Authentication parameters"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
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

	_host := &schema.Host{
		ID: authRequest.ID,
	}
	err := _host.Get(request.Context())
	if err != nil {
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logger.Log(0, request.Header.Get("user"),
			"error retrieving host: ", authRequest.ID, err.Error())
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(_host.Password), []byte(authRequest.Password))
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
	go func() {
		// Create EMQX creds
		if servercfg.GetBrokerType() == servercfg.EmqxBrokerType {
			if err := mq.GetEmqxHandler().CreateEmqxUser(_host.ID, authRequest.Password); err != nil {
				slog.Error("failed to create host credentials for EMQX: ", err.Error())
			}
		}
	}()

	response.WriteHeader(http.StatusOK)
	response.Header().Set("Content-Type", "application/json")
	response.Write(successJSONResponse)
}

// @Summary     Send signal to peer
// @Router      /api/v1/host/{hostid}/signalpeer [post]
// @Tags        Hosts
// @Security    oauth
// @Param       hostid path string true "Host ID"
// @Param       body body models.Signal true "Signal data"
// @Success     200 {object} models.Signal
// @Failure     400 {object} models.ErrorResponse
func signalPeer(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	hostID := params["hostid"]

	// confirm host exists
	_host := &schema.Host{
		ID: hostID,
	}
	err := _host.Get(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to get host:", err.Error())
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		} else {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		}
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
	if signal.ToHostPubKey == "" {
		msg := "insufficient data to signal peer"
		logger.Log(0, r.Header.Get("user"), msg)
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New(msg), "badrequest"))
		return
	}
	signal.IsPro = servercfg.IsPro

	_peerHost := &schema.Host{
		ID: signal.ToHostID,
	}
	err = _peerHost.Get(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("failed to signal, peer not found"), "badrequest"),
		)
		return
	}

	err = mq.HostUpdate(&models.HostUpdate{
		Action: models.SignalHost,
		Host:   converters.ToModelHost(*_peerHost),
		Signal: signal,
	})
	if err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				errors.New("failed to publish signal to peer: "+err.Error()),
				"badrequest",
			),
		)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(signal)
}

// @Summary     Update keys for all hosts
// @Router      /api/hosts/keys [put]
// @Tags        Hosts
// @Security    oauth
// @Success     200 {string} string "OK"
// @Failure     400 {object} models.ErrorResponse
func updateAllKeys(w http.ResponseWriter, r *http.Request) {
	var errorResponse = models.ErrorResponse{}
	w.Header().Set("Content-Type", "application/json")

	_hosts, err := (&schema.Host{}).ListAll(r.Context())
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

		hosts := converters.ToModelHosts(_hosts)
		for _, host := range hosts {
			hostUpdate.Host = host
			logger.Log(2, "updating host", host.ID.String(), " for a key update")
			if err = mq.HostUpdate(&hostUpdate); err != nil {
				logger.Log(
					0,
					"failed to send update to node during a network wide key update",
					host.ID.String(),
					err.Error(),
				)
			}
		}
	}()
	logger.Log(2, r.Header.Get("user"), "updated keys for all hosts")
	w.WriteHeader(http.StatusOK)
}

// @Summary     Update keys for a host
// @Router      /api/hosts/{hostid}/keys [put]
// @Tags        Hosts
// @Security    oauth
// @Param       hostid path string true "Host ID"
// @Success     200 {string} string "OK"
// @Failure     400 {object} models.ErrorResponse
func updateKeys(w http.ResponseWriter, r *http.Request) {
	var errorResponse = models.ErrorResponse{}
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	hostID := params["hostid"]

	_host := &schema.Host{
		ID: hostID,
	}
	err := _host.Get(r.Context())
	if err != nil {
		logger.Log(0, "failed to retrieve host", hostID, err.Error())
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
			Host:   converters.ToModelHost(*_host),
		}
		if err = mq.HostUpdate(&hostUpdate); err != nil {
			logger.Log(0, "failed to send host key update", hostID, err.Error())
		}
	}()

	logger.Log(2, r.Header.Get("user"), "updated key on host", _host.Name)
	w.WriteHeader(http.StatusOK)
}

// @Summary     Requests all the hosts to pull
// @Router      /api/hosts/sync [post]
// @Tags        Hosts
// @Security    oauth
// @Success     200 {string} string "sync all hosts request received"
func syncHosts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	user := r.Header.Get("user")

	go func() {
		slog.Info("requesting all hosts to sync", "user", user)

		_hosts, err := (&schema.Host{}).ListAll(r.Context())
		if err != nil {
			slog.Error("failed to retrieve all hosts", "user", user, "error", err)
			return
		}

		hosts := converters.ToModelHosts(_hosts)
		for _, host := range hosts {
			go func(host models.Host) {
				hostUpdate := models.HostUpdate{
					Action: models.RequestPull,
					Host:   host,
				}
				if err = mq.HostUpdate(&hostUpdate); err != nil {
					slog.Error("failed to request host to sync", "user", user, "host", host.ID.String(), "error", err)
				} else {
					slog.Info("host sync requested", "user", user, "host", host.ID.String())
				}
			}(host)
			time.Sleep(time.Millisecond * 100)
		}
	}()

	slog.Info("sync all hosts request received", "user", user)
	logic.ReturnSuccessResponse(w, r, "sync all hosts request received")
}

// @Summary     Requests a host to pull
// @Router      /api/hosts/{hostid}/sync [post]
// @Tags        Hosts
// @Security    oauth
// @Param       hostid path string true "Host ID"
// @Success     200 {string} string "OK"
// @Failure     400 {object} models.ErrorResponse
func syncHost(w http.ResponseWriter, r *http.Request) {
	hostID := mux.Vars(r)["hostid"]

	var errorResponse = models.ErrorResponse{}
	w.Header().Set("Content-Type", "application/json")

	_host := &schema.Host{
		ID: hostID,
	}
	err := _host.Get(r.Context())
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
			Host:   converters.ToModelHost(*_host),
		}
		if err = mq.HostUpdate(&hostUpdate); err != nil {
			slog.Error("failed to send host pull request", "host", hostID, "error", err)
		}
	}()

	slog.Info("requested host pull", "user", r.Header.Get("user"), "host", hostID)
	w.WriteHeader(http.StatusOK)
}

// @Summary     Deletes all EMQX hosts
// @Router      /api/emqx/hosts [delete]
// @Tags        Hosts
// @Security    oauth
// @Success     200 {string} string "deleted hosts data on emqx"
// @Failure     500 {object} models.ErrorResponse
func delEmqxHosts(w http.ResponseWriter, r *http.Request) {
	_hosts, err := (&schema.Host{}).ListAll(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to fetch hosts: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	for _, _host := range _hosts {
		// delete EMQX credentials for host
		if err := mq.GetEmqxHandler().DeleteEmqxUser(_host.ID); err != nil {
			slog.Error("failed to remove host credentials from EMQX", "id", _host.ID, "error", err)
		}
	}

	err = mq.GetEmqxHandler().DeleteEmqxUser(servercfg.GetMqUserName())
	if err != nil {
		slog.Error(
			"failed to remove server credentials from EMQX",
			"user",
			servercfg.GetMqUserName(),
			"error",
			err,
		)
	}

	logic.ReturnSuccessResponse(w, r, "deleted hosts data on emqx")
}

// @Summary     Fetches host peerinfo
// @Router      /api/host/{hostid}/peer_info [get]
// @Tags        Hosts
// @Security    oauth
// @Param       hostid path string true "Host ID"
// @Success     200 {object} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func getHostPeerInfo(w http.ResponseWriter, r *http.Request) {
	hostID := mux.Vars(r)["hostid"]
	var errorResponse = models.ErrorResponse{}

	_host := &schema.Host{
		ID: hostID,
	}
	err := _host.Get(r.Context())
	if err != nil {
		slog.Error("failed to retrieve host", "error", err)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logic.ReturnErrorResponse(w, r, errorResponse)
		return
	}

	host := converters.ToModelHost(*_host)
	peerInfo, err := logic.GetHostPeerInfo(&host)
	if err != nil {
		slog.Error("failed to retrieve host peerinfo", "error", err)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logic.ReturnErrorResponse(w, r, errorResponse)
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, peerInfo, "fetched host peer info")
}
