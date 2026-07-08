package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/middleware"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
)

func deviceHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/device/register", logic.SecurityCheck(false, http.HandlerFunc(registerDevice))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/device/networks", middleware.Scope(scope.TenantScope, logic.SecurityCheck(false, http.HandlerFunc(getDeviceNetworks)))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/device/networks/{network}/join", middleware.Scope(scope.TenantScope, logic.SecurityCheck(false, http.HandlerFunc(joinDeviceNetwork)))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/device/networks/{network}/leave", middleware.Scope(scope.TenantScope, logic.SecurityCheck(false, http.HandlerFunc(leaveDeviceNetwork)))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/device/networks/{network}/cancel", middleware.Scope(scope.TenantScope, logic.SecurityCheck(false, http.HandlerFunc(cancelDeviceNetworkJoin)))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/device/networks/{network}/exit_nodes", middleware.Scope(scope.TenantScope, logic.SecurityCheck(false, http.HandlerFunc(listDeviceExitNodes)))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/device/networks/{network}/exit_node", middleware.Scope(scope.TenantScope, logic.SecurityCheck(false, http.HandlerFunc(getDeviceExitNode)))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/device/networks/{network}/exit_node", middleware.Scope(scope.TenantScope, logic.SecurityCheck(false, http.HandlerFunc(selectDeviceExitNode)))).
		Methods(http.MethodPut, http.MethodPost)
	r.HandleFunc("/api/v1/device/sync", middleware.Scope(scope.TenantScope, logic.SecurityCheck(false, http.HandlerFunc(syncDevice)))).
		Methods(http.MethodPost)
}

func getDeviceUser(w http.ResponseWriter, r *http.Request) (*schema.User, bool) {
	username := r.Header.Get("user")
	if username == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("user not found in request"), "unauthorized"))
		return nil, false
	}
	user := &schema.User{Username: username}
	if err := user.Get(r.Context()); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "unauthorized"))
		return nil, false
	}
	return user, true
}

func registerDevice(w http.ResponseWriter, r *http.Request) {
	user, ok := getDeviceUser(w, r)
	if !ok {
		return
	}
	var newHost schema.Host
	if err := json.NewDecoder(r.Body).Decode(&newHost); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if newHost.ID == uuid.Nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("host id is required"), "badrequest"))
		return
	}
	if hostID := r.Header.Get(logic.DeviceHostIDHeader); hostID != "" && hostID != newHost.ID.String() {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("host id mismatch"), "badrequest"))
		return
	}
	resp, err := logic.RegisterDevice(db.WithContext(r.Context()), user, &newHost)
	if err != nil {
		errType := logic.Internal
		switch {
		case err.Error() == "host does not belong to user":
			errType = logic.Forbidden
		case err.Error() == "invalid host id", err.Error() == "missing traffic key":
			errType = logic.BadReq
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, errType))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, resp, "device registered")
}

func getDeviceUserAndHost(w http.ResponseWriter, r *http.Request) (*schema.User, *schema.Host, bool) {
	user, ok := getDeviceUser(w, r)
	if !ok {
		return nil, nil, false
	}
	username := user.Username
	hostID := r.Header.Get(logic.DeviceHostIDHeader)
	if hostID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("host id is required"), "badrequest"))
		return nil, nil, false
	}
	host, err := logic.VerifyDeviceHostAccess(r.Context(), username, hostID)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "forbidden"))
		return nil, nil, false
	}
	return user, host, true
}

func getDeviceNetworks(w http.ResponseWriter, r *http.Request) {
	user, ok := getDeviceUser(w, r)
	if !ok {
		return
	}
	var host *schema.Host
	hostID := r.Header.Get(logic.DeviceHostIDHeader)
	if hostID != "" {
		if h, err := logic.VerifyDeviceHostAccess(r.Context(), user.Username, hostID); err == nil {
			host = h
		}
		// Ownership mismatch must not block the network list; host-scoped state is omitted without a verified host.
	}
	networks, err := logic.GetDeviceNetworks(db.WithContext(r.Context()), user, host)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, networks, "fetched device networks")
}

func joinDeviceNetwork(w http.ResponseWriter, r *http.Request) {
	user, host, ok := getDeviceUserAndHost(w, r)
	if !ok {
		return
	}
	network := mux.Vars(r)["network"]
	if network == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network is required"), "badrequest"))
		return
	}
	result, err := logic.JoinDeviceNetwork(db.WithContext(r.Context()), user, host, network)
	if err != nil {
		errType := logic.Internal
		switch err.Error() {
		case "user does not have access to network",
			"JIT access required: please request access from network admin",
			"access blocked: this device doesn't meet security requirements":
			errType = logic.Forbidden
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, errType))
		return
	}
	if result.Status == models.DeviceJoinStatusPending {
		var httpResponse models.SuccessResponse
		httpResponse.Code = http.StatusAccepted
		httpResponse.Response = result
		httpResponse.Message = "host approval pending for network"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(httpResponse)
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, result, "joined network")
}

func leaveDeviceNetwork(w http.ResponseWriter, r *http.Request) {
	user, host, ok := getDeviceUserAndHost(w, r)
	if !ok {
		return
	}
	network := mux.Vars(r)["network"]
	if network == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network is required"), "badrequest"))
		return
	}
	if err := logic.LeaveDeviceNetwork(db.WithContext(r.Context()), user, host, network); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.ReturnSuccessResponse(w, r, "left network")
}

func cancelDeviceNetworkJoin(w http.ResponseWriter, r *http.Request) {
	user, host, ok := getDeviceUserAndHost(w, r)
	if !ok {
		return
	}
	network := mux.Vars(r)["network"]
	if network == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network is required"), "badrequest"))
		return
	}
	if err := logic.CancelDeviceNetworkJoin(db.WithContext(r.Context()), user, host, network); err != nil {
		errType := logic.Internal
		if err.Error() == "no pending join request for network" {
			errType = logic.BadReq
		}
		if err.Error() == "user does not have access to network" {
			errType = logic.Forbidden
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, errType))
		return
	}
	logic.ReturnSuccessResponse(w, r, "join request cancelled")
}

func syncDevice(w http.ResponseWriter, r *http.Request) {
	_, host, ok := getDeviceUserAndHost(w, r)
	if !ok {
		return
	}
	if err := logic.SyncDevice(host); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.ReturnSuccessResponse(w, r, "sync requested")
}

func listDeviceExitNodes(w http.ResponseWriter, r *http.Request) {
	user, host, ok := getDeviceUserAndHost(w, r)
	if !ok {
		return
	}
	network := mux.Vars(r)["network"]
	if network == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network is required"), "badrequest"))
		return
	}
	nodes, err := logic.ListDeviceExitNodes(db.WithContext(r.Context()), user, host, network)
	if err != nil {
		errType := logic.Internal
		switch err.Error() {
		case "user does not have access to network":
			errType = logic.Forbidden
		case "device is not joined to network", "network is required":
			errType = logic.BadReq
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, errType))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, nodes, "fetched exit nodes")
}

func getDeviceExitNode(w http.ResponseWriter, r *http.Request) {
	user, host, ok := getDeviceUserAndHost(w, r)
	if !ok {
		return
	}
	network := mux.Vars(r)["network"]
	if network == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network is required"), "badrequest"))
		return
	}
	selected, err := logic.GetDeviceSelectedExitNode(db.WithContext(r.Context()), user, host, network)
	if err != nil {
		errType := logic.Internal
		switch err.Error() {
		case "user does not have access to network":
			errType = logic.Forbidden
		case "device is not joined to network", "network is required":
			errType = logic.BadReq
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, errType))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, selected, "fetched selected exit node")
}

func selectDeviceExitNode(w http.ResponseWriter, r *http.Request) {
	user, host, ok := getDeviceUserAndHost(w, r)
	if !ok {
		return
	}
	network := mux.Vars(r)["network"]
	if network == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network is required"), "badrequest"))
		return
	}
	var req models.DeviceExitNodeSelectionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	selected, err := logic.SelectDeviceExitNode(db.WithContext(r.Context()), user, host, network, req.EgressID)
	if err != nil {
		errType := logic.Internal
		switch err.Error() {
		case "user does not have access to network", "user does not have access to this exit node":
			errType = logic.Forbidden
		case "device is not joined to network", "network is required", "exit node not found",
			"egress is not an active internet exit node in this network",
			"routing node cannot select itself as exit node":
			errType = logic.BadReq
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, errType))
		return
	}
	msg := "exit node selected"
	if req.EgressID == "" {
		msg = "exit node cleared"
	}
	logic.ReturnSuccessResponseWithJson(w, r, selected, msg)
}
