package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/slog"
	"gorm.io/gorm"
)

func hostHandlers(r *mux.Router) {
	r.HandleFunc("/api/hosts", logic.SecurityCheck(true, http.HandlerFunc(getHosts))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/hosts", logic.SecurityCheck(true, http.HandlerFunc(listHosts))).
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
	r.HandleFunc("/api/hosts/{hostid}", logic.SecurityCheck(true, http.HandlerFunc(getHost))).
		Methods(http.MethodGet)
	// used by netclient
	r.HandleFunc("/api/hosts/{hostid}", AuthorizeHost(http.HandlerFunc(deleteHost))).
		Methods(http.MethodDelete)
	// used by UI
	r.HandleFunc("/api/v1/ui/hosts/{hostid}", logic.SecurityCheck(true, http.HandlerFunc(deleteHost))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/hosts/bulk", logic.SecurityCheck(true, http.HandlerFunc(bulkDeleteHosts))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/hosts/{hostid}/upgrade", logic.SecurityCheck(true, http.HandlerFunc(upgradeHost))).
		Methods(http.MethodPut)
	r.HandleFunc("/api/hosts/{hostid}/networks/{network}", logic.SecurityCheck(true, http.HandlerFunc(addHostToNetwork))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/hosts/{hostid}/networks/{network}", logic.SecurityCheck(true, http.HandlerFunc(deleteHostFromNetwork))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/hosts/adm/authenticate", authenticateHost).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/host", AuthorizeHost(http.HandlerFunc(pull))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/host/{hostid}/signalpeer", AuthorizeHost(http.HandlerFunc(signalPeer))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/fallback/host/{hostid}", AuthorizeHost(http.HandlerFunc(hostUpdateFallback))).
		Methods(http.MethodPut)
	r.HandleFunc("/api/v1/host/{hostid}/peer_info", AuthorizeHost(http.HandlerFunc(getHostPeerInfo))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/pending_hosts", logic.SecurityCheck(true, http.HandlerFunc(getPendingHosts))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/pending_hosts/approve/{id}", logic.SecurityCheck(true, http.HandlerFunc(approvePendingHost))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/pending_hosts/reject/{id}", logic.SecurityCheck(true, http.HandlerFunc(rejectPendingHost))).
		Methods(http.MethodPost)
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

		hosts, err := (&schema.Host{}).ListAll(r.Context())
		if err != nil {
			slog.Error("failed to retrieve all hosts", "user", user, "error", err)
			return
		}

		for _, host := range hosts {
			go func(host schema.Host) {
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
	logic.LogEvent(&models.Event{
		Action: schema.UpgradeAll,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   "All Hosts",
			Name: "All Hosts",
			Type: schema.DeviceSub,
		},
		Origin: schema.Dashboard,
	})
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
	hostIDStr := mux.Vars(r)["hostid"]
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		err = fmt.Errorf("failed to parse host id: %w", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	host := &schema.Host{
		ID: hostID,
	}
	err = host.Get(r.Context())
	if err != nil {
		slog.Error("failed to find host", "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "notfound"))
		return
	}

	action := models.Upgrade

	if r.URL.Query().Get("force") == "true" {
		action = models.ForceUpgrade
	}

	if err := mq.HostUpdate(&models.HostUpdate{Action: action, Host: *host}); err != nil {
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
// @Produce     json
// @Success     200 {array} models.ApiHost
// @Failure     500 {object} models.ErrorResponse
func getHosts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	currentHosts, err := (&schema.Host{}).ListAll(r.Context())
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

// @Summary     List all hosts
// @Router      /api/v1/hosts [get]
// @Tags        Hosts
// @Security    oauth
// @Produce     json
// @Param       os query []string false "Filter by OS" Enums(windows, linux, darwin)
// @Param       q query string false "Search across fields"
// @Param       page query int false "Page number"
// @Param       per_page query int false "Items per page"
// @Success     200 {array} models.ApiHost
// @Failure     500 {object} models.ErrorResponse
func listHosts(w http.ResponseWriter, r *http.Request) {
	var osFilters []interface{}
	for _, filter := range r.URL.Query()["os"] {
		osFilters = append(osFilters, filter)
	}

	q := r.URL.Query().Get("q")

	var page, pageSize int
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if page == 0 {
		page = 1
	}

	pageSize, _ = strconv.Atoi(r.URL.Query().Get("per_page"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	currentHosts, err := (&schema.Host{}).ListAll(
		r.Context(),
		dbtypes.WithFilter("os", osFilters...),
		dbtypes.WithSearchQuery(q, "id", "name", "public_key", "endpoint_ip", "endpoint_ipv6"),
		dbtypes.InAscOrder("name"),
		dbtypes.WithPagination(page, pageSize),
	)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to fetch hosts: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	apiHosts := logic.GetAllHostsAPI(currentHosts[:])
	logger.Log(2, r.Header.Get("user"), "fetched all hosts")

	total, err := (&schema.Host{}).Count(
		r.Context(),
		dbtypes.WithFilter("os", osFilters...),
		dbtypes.WithSearchQuery(q, "id", "name", "public_key", "endpoint_ip", "endpoint_ipv6"),
	)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	response := models.PaginatedResponse{
		Data:       apiHosts,
		Page:       page,
		PerPage:    pageSize,
		Total:      total,
		TotalPages: totalPages,
	}

	logic.ReturnSuccessResponseWithJson(w, r, response, "fetched hosts")
}

// @Summary     Used by clients for "pull" command
// @Router      /api/v1/host [get]
// @Tags        Hosts
// @Security    oauth
// @Produce     json
// @Success     200 {object} models.HostPull
// @Failure     500 {object} models.ErrorResponse
func pull(w http.ResponseWriter, r *http.Request) {
	hostIDStr := r.Header.Get(hostIDHeader) // return JSON/API formatted keys
	if len(hostIDStr) == 0 {
		logger.Log(0, "no host authorized to pull")
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(fmt.Errorf("no host authorized to pull"), "internal"),
		)
		return
	}

	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		err = fmt.Errorf("failed to parse host id: %w", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	host := &schema.Host{
		ID: hostID,
	}
	err = host.Get(r.Context())
	if err != nil {
		logger.Log(0, "no host found during pull", hostIDStr)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	resetFailovered := r.URL.Query().Get("reset_failovered") == "true"
	if resetFailovered {
		for _, nodeID := range host.Nodes {
			node, err := logic.GetNodeByID(nodeID)
			if err != nil {
				continue
			}
			logic.ResetFailedOverPeer(&node)
			logic.ResetAutoRelayedPeer(&node)
		}
		go mq.PublishPeerUpdate(false)
	}

	hPU, ok := logic.GetCachedHostPeerUpdate(hostID.String())
	if !ok || resetFailovered {
		allNodes, err := logic.GetAllNodes()
		if err != nil {
			logger.Log(0, "failed to get nodes: ", hostID.String())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		hPU, err = logic.GetPeerUpdateForHost("", host, allNodes, nil, nil)
		if err != nil {
			logger.Log(0, "could not pull peers for host", hostID.String(), err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		logic.StoreHostPeerUpdate(hostID.String(), hPU)
	}

	response := models.HostPull{
		Host:               *host,
		Nodes:              logic.GetHostNodes(host),
		ServerConfig:       hPU.ServerConfig,
		Peers:              hPU.Peers,
		PeerIDs:            hPU.PeerIDs,
		HostNetworkInfo:    hPU.HostNetworkInfo,
		EgressRoutes:       hPU.EgressRoutes,
		FwUpdate:           hPU.FwUpdate,
		ChangeDefaultGw:    hPU.ChangeDefaultGw,
		DefaultGwIp:        hPU.DefaultGwIp,
		IsInternetGw:       hPU.IsInternetGw,
		NameServers:        hPU.NameServers,
		EgressWithDomains:  hPU.EgressWithDomains,
		EndpointDetection:  logic.IsEndpointDetectionEnabled(),
		DnsNameservers:     hPU.DnsNameservers,
		ReplacePeers:       hPU.ReplacePeers,
		AutoRelayNodes:     make(map[schema.NetworkID][]models.Node),
		GwNodes:            make(map[schema.NetworkID][]models.Node),
		AddressIdentityMap: hPU.AddressIdentityMap,
	}

	logger.Log(1, hostIDStr, host.Name, "completed a pull")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&response)
}

// @Summary     Updates a Netclient host on Netmaker server
// @Router      /api/hosts/{hostid} [put]
// @Tags        Hosts
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       hostid path string true "Host ID"
// @Param       body body models.ApiHost true "New host data"
// @Success     200 {object} models.ApiHost
// @Failure     500 {object} models.ErrorResponse
func updateHost(w http.ResponseWriter, r *http.Request) {
	var newHostData models.ApiHost
	err := json.NewDecoder(r.Body).Decode(&newHostData)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	hostID, err := uuid.Parse(newHostData.ID)
	if err != nil {
		err = fmt.Errorf("failed to parse host id: %w", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	// confirm host exists
	currHost := &schema.Host{
		ID: hostID,
	}
	err = currHost.Get(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	newHost := newHostData.ConvertAPIHostToNMHost(currHost)

	logic.UpdateHost(newHost, currHost) // update the in memory struct values
	if newHost.DNS != "yes" {
		// check if any node is internet gw
		for _, nodeID := range newHost.Nodes {
			node, err := logic.GetNodeByID(nodeID)
			if err != nil {
				continue
			}
			if node.IsInternetGateway {
				newHost.DNS = "yes"
				break
			}
		}
	}
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
		logger.Log(
			0,
			r.Header.Get("user"),
			"failed to send host update: ",
			currHost.ID.String(),
			err.Error(),
		)
	}
	go func() {
		if newHost.IsDefault && !currHost.IsDefault {
			addDefaultHostToNetworks(newHost)
		}
		if err := mq.PublishPeerUpdate(false); err != nil {
			logger.Log(0, "fail to publish peer update: ", err.Error())
		}
		if newHost.Name != currHost.Name {
			if servercfg.IsDNSMode() {
				logic.SetDNS()
			}
		}
	}()

	logic.LogEvent(&models.Event{
		Action: schema.Update,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   currHost.ID.String(),
			Name: newHost.Name,
			Type: schema.DeviceSub,
		},
		Diff: models.Diff{
			Old: currHost,
			New: newHost,
		},
		Origin: schema.Dashboard,
	})
	apiHostData := models.NewApiHostFromSchemaHost(newHost)
	logger.Log(2, r.Header.Get("user"), "updated host", newHost.ID.String())
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiHostData)
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
	var params = mux.Vars(r)
	hostIDStr := params["hostid"]
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		err = fmt.Errorf("failed to parse host id: %w", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}
	currentHost := &schema.Host{
		ID: hostID,
	}
	err = currentHost.Get(r.Context())
	if err != nil {
		slog.Error("error getting host", "id", hostIDStr, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	var sendPeerUpdate, sendDeletedNodeUpdate, replacePeers bool
	var hostUpdate models.HostUpdate
	err = json.NewDecoder(r.Body).Decode(&hostUpdate)
	if err != nil {
		slog.Error("failed to update a host:", "user", r.Header.Get("user"), "error", err.Error(), "host", currentHost.Name)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	slog.Info("recieved host update", "name", hostUpdate.Host.Name, "id", hostUpdate.Host.ID, "action", hostUpdate.Action)
	switch hostUpdate.Action {
	case models.CheckIn:
		sendPeerUpdate = mq.HandleHostCheckin(&hostUpdate.Host, currentHost)
	case models.UpdateHost:
		if hostUpdate.Host.PublicKey != currentHost.PublicKey {
			//remove old peer entry
			replacePeers = true
		}
		var endpointChanged bool
		endpointChanged, sendPeerUpdate = logic.UpdateHostFromClient(&hostUpdate.Host, currentHost)
		if endpointChanged {
			logic.CheckHostPorts(currentHost)
		}
		err := logic.UpsertHost(currentHost)
		if err != nil {
			slog.Error("failed to update host", "id", currentHost.ID, "error", err)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
			return
		}
	case models.UpdateNode:
		var displacedGwNodes []models.Node
		sendDeletedNodeUpdate, sendPeerUpdate, displacedGwNodes = logic.UpdateHostNode(&hostUpdate.Host, &hostUpdate.Node)
		if len(displacedGwNodes) > 0 {
			go func() {
				for _, dNode := range displacedGwNodes {
					dHost := &schema.Host{ID: dNode.HostID}
					if err := dHost.Get(db.WithContext(context.TODO())); err != nil {
						slog.Error("fallback disconnect gw: failed to get host for displaced node", "node", dNode.ID, "error", err)
						continue
					}
					mq.HostUpdate(&models.HostUpdate{Action: models.CheckAutoAssignGw, Host: *dHost, Node: dNode})
				}
			}()
		}
	case models.UpdateMetrics:
		mq.UpdateMetricsFallBack(hostUpdate.Node.ID.String(), hostUpdate.NewMetrics)
	case models.EgressUpdate:
		e := schema.Egress{ID: hostUpdate.EgressDomain.ID}
		err = e.Get(db.WithContext(r.Context()))
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
			return
		}
		if len(hostUpdate.Node.EgressGatewayRanges) > 0 {
			e.DomainAns = hostUpdate.Node.EgressGatewayRanges
			e.Update(db.WithContext(r.Context()))
		}
		sendPeerUpdate = true
	case models.SignalHost:
		mq.SignalPeer(hostUpdate.Signal)
	case models.DeleteHost:
		go mq.DeleteAndCleanupHost(currentHost)
	}
	go func() {
		if sendDeletedNodeUpdate {
			mq.PublishDeletedNodePeerUpdate(&hostUpdate.Node)
		}
		if sendPeerUpdate {
			err := mq.PublishPeerUpdate(replacePeers)
			if err != nil {
				slog.Error("failed to publish peer update", "error", err)
			}
		}
	}()

	logic.ReturnSuccessResponse(w, r, "updated host data")
}

// @Summary     Deletes a Netclient host from Netmaker server
// @Router      /api/hosts/{hostid} [delete]
// @Tags        Hosts
// @Security    oauth
// @Produce     json
// @Param       hostid path string true "Host ID"
// @Param       force query bool false "Force delete"
// @Success     200 {object} models.ApiHost
// @Failure     500 {object} models.ErrorResponse
func deleteHost(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	hostIDStr := params["hostid"]
	forceDelete := r.URL.Query().Get("force") == "true"
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		err = fmt.Errorf("failed to parse host id: %w", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}
	// confirm host exists
	currHost := &schema.Host{
		ID: hostID,
	}
	err = currHost.Get(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to delete a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	var hostNodes []models.Node
	for _, nodeID := range currHost.Nodes {
		node, err := logic.GetNodeByID(nodeID)
		if err != nil {
			slog.Error("failed to get node", "nodeid", nodeID, "error", err)
			continue
		}
		hostNodes = append(hostNodes, node)
	}
	if err = logic.RemoveHost(currHost, forceDelete); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to delete a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	for _, node := range hostNodes {
		go mq.PublishMqUpdatesForDeletedNode(node, false)
	}
	if servercfg.GetBrokerType() == servercfg.EmqxBrokerType {
		if err := mq.GetEmqxHandler().DeleteEmqxUser(currHost.ID.String()); err != nil {
			slog.Error(
				"failed to remove host credentials from EMQX",
				"id",
				currHost.ID,
				"error",
				err,
			)
		}
	}
	if err = mq.HostUpdate(&models.HostUpdate{
		Action: models.DeleteHost,
		Host:   *currHost,
	}); err != nil {
		logger.Log(
			0,
			r.Header.Get("user"),
			"failed to send delete host update: ",
			currHost.ID.String(),
			err.Error(),
		)
	}
	// delete if any pending reqs
	(&schema.PendingHost{
		HostID: currHost.ID.String(),
	}).DeleteAllPendingHosts(db.WithContext(r.Context()))
	logic.LogEvent(&models.Event{
		Action: schema.Delete,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   currHost.ID.String(),
			Name: currHost.Name,
			Type: schema.DeviceSub,
		},
		Origin: schema.Dashboard,
		Diff: models.Diff{
			Old: currHost,
			New: nil,
		},
	})
	apiHostData := models.NewApiHostFromSchemaHost(currHost)
	logger.Log(2, r.Header.Get("user"), "removed host", currHost.Name)
	logic.ReturnSuccessResponseWithJson(w, r, apiHostData, "deleted host "+currHost.Name)
}

// @Summary     Fetches a Netclient host from Netmaker server
// @Router      /api/hosts/{hostid} [get]
// @Tags        Hosts
// @Security    oauth
// @Produce     json
// @Param       hostid path string true "Host ID"
// @Success     200 {object} models.ApiHost
// @Failure     500 {object} models.ErrorResponse
func getHost(w http.ResponseWriter, r *http.Request) {
	hostIDStr := mux.Vars(r)["hostid"]
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		err = fmt.Errorf("failed to parse host id: %w", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}
	host := &schema.Host{
		ID: hostID,
	}
	err = host.Get(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to fetch a host:", err.Error())

		apiErr := logic.Internal
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiErr = logic.NotFound
		}

		logic.ReturnErrorResponse(w, r, logic.FormatError(err, apiErr))
		return
	}
	apiHostData := models.NewApiHostFromSchemaHost(host)
	logic.ReturnSuccessResponseWithJson(w, r, apiHostData, "fetched host "+host.Name)
}

// @Summary     Bulk delete hosts
// @Router      /api/v1/hosts/bulk [delete]
// @Tags        Hosts
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       body body models.BulkDeleteRequest true "List of host IDs to delete"
// @Success     202 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
func bulkDeleteHosts(w http.ResponseWriter, r *http.Request) {
	var req models.BulkDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("invalid request body: %w", err), logic.BadReq))
		return
	}
	if len(req.IDs) == 0 {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("no host IDs provided"), logic.BadReq))
		return
	}
	user := r.Header.Get("user")
	logic.ReturnAcceptedResponse(w, r, fmt.Sprintf("bulk delete of %d host(s) accepted", len(req.IDs)))

	go func() {
		deleted := 0
		for _, idStr := range req.IDs {
			hostID, err := uuid.Parse(idStr)
			if err != nil {
				slog.Debug("bulk host delete: invalid host id", "id", idStr)
				continue
			}
			currHost := &schema.Host{ID: hostID}
			if err = currHost.Get(db.WithContext(context.Background())); err != nil {
				slog.Debug("bulk host delete: host not found", "id", idStr, "error", err)
				continue
			}
			var hostNodes []models.Node
			for _, nodeID := range currHost.Nodes {
				node, err := logic.GetNodeByID(nodeID)
				if err != nil {
					slog.Debug("bulk host delete: failed to get node", "nodeid", nodeID, "error", err)
					continue
				}
				hostNodes = append(hostNodes, node)
			}
			if err = logic.RemoveHost(currHost, true); err != nil {
				slog.Debug("bulk host delete: failed to remove host", "id", idStr, "error", err)
				continue
			}
			for _, node := range hostNodes {
				go mq.PublishMqUpdatesForDeletedNode(node, false)
			}
			if servercfg.GetBrokerType() == servercfg.EmqxBrokerType {
				if err := mq.GetEmqxHandler().DeleteEmqxUser(currHost.ID.String()); err != nil {
					slog.Debug("bulk host delete: failed to remove EMQX credentials", "id", currHost.ID, "error", err)
				}
			}
			if err = mq.HostUpdate(&models.HostUpdate{
				Action: models.DeleteHost,
				Host:   *currHost,
			}); err != nil {
				slog.Debug("bulk host delete: failed to send host update", "id", currHost.ID, "error", err)
			}
			(&schema.PendingHost{HostID: currHost.ID.String()}).DeleteAllPendingHosts(db.WithContext(context.TODO()))
			logic.LogEvent(&models.Event{
				Action: schema.Delete,
				Source: models.Subject{
					ID:   user,
					Name: user,
					Type: schema.UserSub,
				},
				TriggeredBy: user,
				Target: models.Subject{
					ID:   currHost.ID.String(),
					Name: currHost.Name,
					Type: schema.DeviceSub,
				},
				Origin: schema.Dashboard,
				Diff:   models.Diff{Old: currHost, New: nil},
			})
			logger.Log(2, user, "removed host", currHost.Name)
			deleted++
		}
		if deleted > 0 {
			if err := mq.PublishPeerUpdate(false); err != nil {
				slog.Error("bulk host delete: failed to publish peer update", "error", err)
			}
		}
		slog.Info("bulk host delete completed", "deleted", deleted, "total", len(req.IDs))
	}()
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
	hostIDStr := params["hostid"]
	network := params["network"]
	if hostIDStr == "" || network == "" {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("hostid or network cannot be empty"), logic.BadReq),
		)
		return
	}
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		err = fmt.Errorf("failed to parse host id: %w", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	// confirm host exists
	currHost := &schema.Host{
		ID: hostID,
	}
	err = currHost.Get(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to find host:", hostIDStr, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	violations, _ := logic.CheckPostureViolations(models.PostureCheckDeviceInfo{
		ClientLocation: currHost.CountryCode,
		ClientVersion:  currHost.Version,
		OS:             currHost.OS,
		OSFamily:       currHost.OSFamily,
		OSVersion:      currHost.OSVersion,
		KernelVersion:  currHost.KernelVersion,

		SkipAutoUpdate: true,
	}, schema.NetworkID(network))
	if len(violations) > 0 {
		logic.ReturnErrorResponseWithJson(w, r, violations, logic.FormatError(errors.New("posture check violations"), logic.BadReq))
		return
	}
	newNode, err := logic.UpdateHostNetwork(currHost, network, true)
	if err != nil {
		logger.Log(
			0,
			r.Header.Get("user"),
			"failed to add host to network:",
			hostIDStr,
			network,
			err.Error(),
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(1, "added new node", newNode.ID.String(), "to host", currHost.Name)
	if currHost.IsDefault {
		// make host gateway
		logic.CreateIngressGateway(network, newNode.ID.String(), models.IngressRequest{})
		logic.CreateRelay(models.RelayRequest{
			NodeID: newNode.ID.String(),
			NetID:  network,
		})
	}
	go func() {
		mq.HostUpdate(&models.HostUpdate{
			Action: models.JoinHostToNetwork,
			Host:   *currHost,
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
		fmt.Sprintf("added host %s to network %s", currHost.Name, network),
	)
	logic.LogEvent(&models.Event{
		Action: schema.JoinHostToNet,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   currHost.ID.String(),
			Name: currHost.Name,
			Type: schema.DeviceSub,
		},
		NetworkID: schema.NetworkID(network),
		Origin:    schema.Dashboard,
	})
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
	hostIDStr := params["hostid"]
	network := params["network"]
	forceDelete := r.URL.Query().Get("force") == "true"
	if hostIDStr == "" || network == "" {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("hostid or network cannot be empty"), "badrequest"),
		)
		return
	}
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		err = fmt.Errorf("failed to parse host id: %w", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}
	// confirm host exists
	currHost := &schema.Host{
		ID: hostID,
	}
	err = currHost.Get(r.Context())
	if err != nil {
		if database.IsEmptyRecord(err) {
			// check if there is any daemon nodes that needs to be deleted
			node, err := logic.GetNodeByHostRef(hostIDStr, network)
			if err != nil {
				slog.Error(
					"couldn't get node for host",
					"hostid",
					hostIDStr,
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
					"nodeid", node.ID.String(), "hostid", hostIDStr, "network", network, "error", err)
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

	node, err := logic.UpdateHostNetwork(currHost, network, false)
	if err != nil {
		if node == nil && forceDelete {
			// force cleanup the node
			node, err := logic.GetNodeByHostRef(hostIDStr, network)
			if err != nil {
				slog.Error(
					"couldn't get node for host",
					"hostid",
					hostIDStr,
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
					"nodeid", node.ID.String(), "hostid", hostIDStr, "network", network, "error", err)
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
			hostIDStr,
			network,
			err.Error(),
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(1, "deleting node", node.ID.String(), "from host", currHost.Name)
	if err := logic.DeleteNode(node, forceDelete); err != nil {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(fmt.Errorf("failed to delete node"), "internal"),
		)
		return
	}
	go func() {
		mq.PublishMqUpdatesForDeletedNode(*node, true)
		if servercfg.IsDNSMode() {
			logic.SetDNS()
		}
	}()
	logic.LogEvent(&models.Event{
		Action: schema.RemoveHostFromNet,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   currHost.ID.String(),
			Name: currHost.Name,
			Type: schema.DeviceSub,
		},
		NetworkID: schema.NetworkID(network),
		Origin:    schema.Dashboard,
	})
	logger.Log(
		2,
		r.Header.Get("user"),
		fmt.Sprintf("removed host %s from network %s", currHost.Name, network),
	)
	w.WriteHeader(http.StatusOK)
}

// @Summary     To Fetch Auth Token for a Host
// @Router      /api/hosts/adm/authenticate [post]
// @Tags        Auth
// @Accept      json
// @Produce     json
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
	hostID, err := uuid.Parse(authRequest.ID)
	if err != nil {
		err = fmt.Errorf("failed to parse host id: %w", err)
		logic.ReturnErrorResponse(response, request, logic.FormatError(err, logic.BadReq))
		return
	}

	host := &schema.Host{
		ID: hostID,
	}
	err = host.Get(request.Context())
	if err != nil {
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logger.Log(0, request.Header.Get("user"),
			"error retrieving host: ", authRequest.ID, err.Error())
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
	go func() {
		// Create EMQX creds
		if servercfg.GetBrokerType() == servercfg.EmqxBrokerType {
			if err := mq.GetEmqxHandler().CreateEmqxUser(host.ID.String(), authRequest.Password); err != nil {
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
	hostIDStr := params["hostid"]
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		err = fmt.Errorf("failed to parse host id: %w", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}
	// confirm host exists
	err = (&schema.Host{
		ID: hostID,
	}).Get(r.Context())
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
	if signal.ToHostPubKey == "" {
		msg := "insufficient data to signal peer"
		logger.Log(0, r.Header.Get("user"), msg)
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New(msg), "badrequest"))
		return
	}
	signal.IsPro = servercfg.IsPro
	hostID, err = uuid.Parse(signal.ToHostID)
	if err != nil {
		err = fmt.Errorf("failed to parse host id: %w", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}
	peerHost := &schema.Host{
		ID: hostID,
	}
	err = peerHost.Get(r.Context())
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
		Host:   *peerHost,
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
	hosts, err := (&schema.Host{}).ListAll(r.Context())
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
				logger.Log(
					0,
					"failed to send update to node during a network wide key update",
					host.ID.String(),
					err.Error(),
				)
			}
		}
	}()
	logic.LogEvent(&models.Event{
		Action: schema.RefreshAllKeys,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   "All Devices",
			Name: "All Devices",
			Type: schema.DeviceSub,
		},
		Origin: schema.Dashboard,
	})
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
	hostIDStr := params["hostid"]
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		err = fmt.Errorf("failed to parse host id: %w", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}
	host := &schema.Host{
		ID: hostID,
	}
	err = host.Get(r.Context())
	if err != nil {
		logger.Log(0, "failed to retrieve host", hostIDStr, err.Error())
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
	logic.LogEvent(&models.Event{
		Action: schema.RefreshKey,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   host.ID.String(),
			Name: host.Name,
			Type: schema.DeviceSub,
		},
		Origin: schema.Dashboard,
	})
	logger.Log(2, r.Header.Get("user"), "updated key on host", host.Name)
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

		hosts, err := (&schema.Host{}).ListAll(r.Context())
		if err != nil {
			slog.Error("failed to retrieve all hosts", "user", user, "error", err)
			return
		}

		for _, host := range hosts {
			go func(host schema.Host) {
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
	logic.LogEvent(&models.Event{
		Action: schema.SyncAll,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   "All Devices",
			Name: "All Devices",
			Type: schema.DeviceSub,
		},
		Origin: schema.Dashboard,
	})
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
	hostIDStr := mux.Vars(r)["hostid"]

	var errorResponse = models.ErrorResponse{}
	w.Header().Set("Content-Type", "application/json")
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		err = fmt.Errorf("failed to parse host id: %w", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}
	host := &schema.Host{
		ID: hostID,
	}
	err = host.Get(r.Context())
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
	logic.LogEvent(&models.Event{
		Action: schema.Sync,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   host.ID.String(),
			Name: host.Name,
			Type: schema.DeviceSub,
		},
		Origin: schema.Dashboard,
	})
	slog.Info("requested host pull", "user", r.Header.Get("user"), "host", host.ID.String())
	w.WriteHeader(http.StatusOK)
}

func delEmqxHosts(w http.ResponseWriter, r *http.Request) {
	currentHosts, err := (&schema.Host{}).ListAll(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to fetch hosts: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	for _, host := range currentHosts {
		// delete EMQX credentials for host
		if err := mq.GetEmqxHandler().DeleteEmqxUser(host.ID.String()); err != nil {
			slog.Error("failed to remove host credentials from EMQX", "id", host.ID, "error", err)
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
// @Router      /api/v1/host/{hostid}/peer_info [get]
// @Tags        Hosts
// @Security    oauth
// @Produce     json
// @Param       hostid path string true "Host ID"
// @Success     200 {object} models.HostPeerInfo
// @Failure     500 {object} models.ErrorResponse
func getHostPeerInfo(w http.ResponseWriter, r *http.Request) {
	hostIDStr := mux.Vars(r)["hostid"]
	var errorResponse = models.ErrorResponse{}
	hostID, err := uuid.Parse(hostIDStr)
	if err != nil {
		err = fmt.Errorf("failed to parse host id: %w", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}
	host := &schema.Host{
		ID: hostID,
	}
	err = host.Get(r.Context())
	if err != nil {
		slog.Error("failed to retrieve host", "error", err)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logic.ReturnErrorResponse(w, r, errorResponse)
		return
	}
	peerInfo, err := logic.GetHostPeerInfo(host)
	if err != nil {
		slog.Error("failed to retrieve host peerinfo", "error", err)
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logic.ReturnErrorResponse(w, r, errorResponse)
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, peerInfo, "fetched host peer info")
}

// @Summary     List pending hosts in a network
// @Router      /api/v1/pending_hosts [get]
// @Tags        Hosts
// @Security    oauth
// @Produce     json
// @Param       network query string true "Network ID"
// @Success     200 {array} schema.PendingHost
// @Failure     500 {object} models.ErrorResponse
func getPendingHosts(w http.ResponseWriter, r *http.Request) {
	netID := r.URL.Query().Get("network")
	if netID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network id param is missing"), "badrequest"))
		return
	}
	pendingHosts, err := (&schema.PendingHost{
		Network: netID,
	}).List(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}
	logger.Log(2, r.Header.Get("user"), "fetched all hosts")
	logic.ReturnSuccessResponseWithJson(w, r, pendingHosts, "returned pending hosts in "+netID)
}

// @Summary     Approve pending host in a network
// @Router      /api/v1/pending_hosts/approve/{id} [post]
// @Tags        Hosts
// @Security    oauth
// @Produce     json
// @Param       id path string true "Pending Host ID"
// @Success     200 {object} models.ApiNode
// @Failure     500 {object} models.ErrorResponse
func approvePendingHost(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	p := &schema.PendingHost{ID: id}
	err := p.Get(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}
	hostID, err := uuid.Parse(p.HostID)
	if err != nil {
		err = fmt.Errorf("failed to parse host id: %w", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}
	h := &schema.Host{
		ID: hostID,
	}
	err = h.Get(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}
	key := models.EnrollmentKey{}
	json.Unmarshal(p.EnrollmentKey, &key)
	newNode, err := logic.UpdateHostNetwork(h, p.Network, true)
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}
	if key.AutoAssignGateway {
		newNode.AutoAssignGateway = true
	}
	if len(key.Groups) > 0 {
		newNode.Tags = make(map[models.TagID]struct{})
		for _, tagI := range key.Groups {
			newNode.Tags[tagI] = struct{}{}
		}
	}
	if key.Relay != uuid.Nil && !newNode.IsRelayed {
		// check if relay node exists and acting as relay
		relaynode, err := logic.GetNodeByID(key.Relay.String())
		if err == nil && relaynode.IsGw && relaynode.Network == newNode.Network {
			slog.Error(fmt.Sprintf("adding relayed node %s to relay %s on network %s", newNode.ID.String(), key.Relay.String(), p.Network))
			newNode.IsRelayed = true
			newNode.RelayedBy = key.Relay.String()
			updatedRelayNode := relaynode
			updatedRelayNode.RelayedNodes = append(updatedRelayNode.RelayedNodes, newNode.ID.String())
			logic.UpdateRelayed(&relaynode, &updatedRelayNode)
			if err := logic.UpsertNode(&updatedRelayNode); err != nil {
				slog.Error("failed to update node", "nodeid", key.Relay.String())
			}
		} else {
			slog.Error("failed to relay node. maybe specified relay node is actually not a relay? Or the relayed node is not in the same network with relay?", "err", err)
		}
	}

	err = logic.UpsertNode(newNode)
	if err != nil {
		err = fmt.Errorf("failed to update node: %w", err)
		slog.Error("failed to update node", "nodeid", newNode.ID.String())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	logger.Log(1, "added new node", newNode.ID.String(), "to host", h.Name)
	mq.HostUpdate(&models.HostUpdate{
		Action: models.JoinHostToNetwork,
		Host:   *h,
		Node:   *newNode,
	})
	if h.IsDefault {
		// make host gateway
		logic.CreateIngressGateway(p.Network, newNode.ID.String(), models.IngressRequest{})
		logic.CreateRelay(models.RelayRequest{
			NodeID: newNode.ID.String(),
			NetID:  p.Network,
		})
	}
	p.Delete(db.WithContext(r.Context()))
	go mq.PublishPeerUpdate(false)
	logic.ReturnSuccessResponseWithJson(w, r, newNode.ConvertToAPINode(), "added pending host to "+p.Network)
}

// @Summary     Reject pending host in a network
// @Router      /api/v1/pending_hosts/reject/{id} [post]
// @Tags        Hosts
// @Security    oauth
// @Produce     json
// @Param       id path string true "Pending Host ID"
// @Success     200 {object} schema.PendingHost
// @Failure     500 {object} models.ErrorResponse
func rejectPendingHost(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	p := &schema.PendingHost{ID: id}
	err := p.Get(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}
	err = p.Delete(db.WithContext(r.Context()))
	if err != nil {
		logic.ReturnErrorResponse(w, r, models.ErrorResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, p, "deleted pending host from "+p.Network)
}

// addDefaultHostToNetworks enrolls a newly-made-default host into every
// existing network it is not already part of, applying the standard default
// host operations for each network.
func addDefaultHostToNetworks(host *schema.Host) {
	networks, err := (&schema.Network{}).ListAll(db.WithContext(context.TODO()))
	if err != nil {
		logger.Log(0, "failed to get networks for default host ops:", err.Error())
		return
	}
	for _, network := range networks {
		if !network.AutoJoin {
			continue
		}
		newNode, err := logic.UpdateHostNetwork(host, network.Name, true)
		if err != nil {
			logger.Log(2, "skipping network", network.Name, "for default host", host.Name, ":", err.Error())
			continue
		}
		logger.Log(1, "added default host", host.Name, "to network", network.Name)
		if len(host.Nodes) == 1 {
			mq.HostUpdate(&models.HostUpdate{
				Action: models.RequestPull,
				Host:   *host,
				Node:   *newNode,
			})
		} else {
			mq.HostUpdate(&models.HostUpdate{
				Action: models.JoinHostToNetwork,
				Host:   *host,
				Node:   *newNode,
			})
		}
		logic.CreateIngressGateway(network.Name, newNode.ID.String(), models.IngressRequest{})
		logic.CreateRelay(models.RelayRequest{
			NodeID: newNode.ID.String(),
			NetID:  network.Name,
		})
	}
}
