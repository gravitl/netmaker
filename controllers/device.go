package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func deviceHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/device/networks", logic.SecurityCheck(false, http.HandlerFunc(getDeviceNetworks))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/device/networks/{network}/join", logic.SecurityCheck(false, http.HandlerFunc(joinDeviceNetwork))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/device/networks/{network}/leave", logic.SecurityCheck(false, http.HandlerFunc(leaveDeviceNetwork))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/device/networks/{network}/jit/request", logic.SecurityCheck(false, http.HandlerFunc(requestDeviceJITAccess))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/device/sync", logic.SecurityCheck(false, http.HandlerFunc(syncDevice))).
		Methods(http.MethodPost)
}

func getDeviceUserAndHost(w http.ResponseWriter, r *http.Request) (*schema.User, *schema.Host, bool) {
	username := r.Header.Get("user")
	if username == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("user not found in request"), "unauthorized"))
		return nil, nil, false
	}
	user := &schema.User{Username: username}
	if err := user.Get(r.Context()); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "unauthorized"))
		return nil, nil, false
	}
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
	user, host, ok := getDeviceUserAndHost(w, r)
	if !ok {
		return
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
	if err := logic.JoinDeviceNetwork(db.WithContext(r.Context()), user, host, network); err != nil {
		errType := logic.Internal
		switch err.Error() {
		case "user does not have access to network",
			"JIT access required: please request access from network admin",
			"access blocked: this device doesn't meet security requirements":
			errType = logic.Forbidden
		case "host approval required for network", "host approval pending for network":
			errType = logic.BadReq
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, errType))
		return
	}
	logic.ReturnSuccessResponse(w, r, "joined network")
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

func requestDeviceJITAccess(w http.ResponseWriter, r *http.Request) {
	user, _, ok := getDeviceUserAndHost(w, r)
	if !ok {
		return
	}
	network := mux.Vars(r)["network"]
	if network == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network is required"), "badrequest"))
		return
	}
	var req models.DeviceJITAccessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if req.Reason == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("reason is required"), "badrequest"))
		return
	}
	result, err := logic.RequestDeviceJITAccess(user, network, req.Reason)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "forbidden"))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, result, "jit access requested")
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
