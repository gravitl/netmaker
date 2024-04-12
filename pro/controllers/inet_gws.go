package controllers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/servercfg"
)

// InetHandlers - handlers for internet gw
func InetHandlers(r *mux.Router) {
	r.HandleFunc("/api/nodes/{network}/{nodeid}/inet_gw", logic.SecurityCheck(true, http.HandlerFunc(createInternetGw))).Methods(http.MethodPost)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/inet_gw", logic.SecurityCheck(true, http.HandlerFunc(updateInternetGw))).Methods(http.MethodPut)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/inet_gw", logic.SecurityCheck(true, http.HandlerFunc(deleteInternetGw))).Methods(http.MethodDelete)
}

// swagger:route POST /api/nodes/{network}/{nodeid}/inet_gw nodes createInternetGw
//
// Create an inet node.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
func createInternetGw(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	nodeid := params["nodeid"]
	netid := params["network"]
	node, err := logic.ValidateParams(nodeid, netid)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if node.IsInternetGateway {
		logic.ReturnSuccessResponse(w, r, "node is already acting as internet gateway")
		return
	}
	var request models.InetNodeReq
	err = json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	host, err := logic.GetHost(node.HostID.String())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if host.OS != models.OS_Types.Linux {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("only linux nodes can be made internet gws"), "badrequest"))
		return
	}
	err = proLogic.ValidateInetGwReq(node, request, false)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	proLogic.SetInternetGw(&node, request)
	if servercfg.IsPro {
		if _, exists := proLogic.FailOverExists(node.Network); exists {
			go func() {
				proLogic.ResetFailedOverPeer(&node)
				mq.PublishPeerUpdate(false)
			}()
		}
	}
	err = logic.UpsertNode(&node)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	apiNode := node.ConvertToAPINode()
	logger.Log(1, r.Header.Get("user"), "created ingress gateway on node", nodeid, "on network", netid)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
	go mq.PublishPeerUpdate(false)
}

// swagger:route PUT /api/nodes/{network}/{nodeid}/inet_gw nodes updateInternetGw
//
// update an inet node.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
func updateInternetGw(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	nodeid := params["nodeid"]
	netid := params["network"]
	node, err := logic.ValidateParams(nodeid, netid)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	var request models.InetNodeReq
	err = json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if !node.IsInternetGateway {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("node is not a internet gw"), "badrequest"))
		return
	}
	err = proLogic.ValidateInetGwReq(node, request, true)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	proLogic.UnsetInternetGw(&node)
	proLogic.SetInternetGw(&node, request)
	err = logic.UpsertNode(&node)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	apiNode := node.ConvertToAPINode()
	logger.Log(1, r.Header.Get("user"), "created ingress gateway on node", nodeid, "on network", netid)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
	go mq.PublishPeerUpdate(false)
}

// swagger:route DELETE /api/nodes/{network}/{nodeid}/inet_gw nodes deleteInternetGw
//
// Delete an internet gw.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
func deleteInternetGw(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	nodeid := params["nodeid"]
	netid := params["network"]
	node, err := logic.ValidateParams(nodeid, netid)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	proLogic.UnsetInternetGw(&node)
	err = logic.UpsertNode(&node)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	apiNode := node.ConvertToAPINode()
	logger.Log(1, r.Header.Get("user"), "created ingress gateway on node", nodeid, "on network", netid)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
	go mq.PublishPeerUpdate(false)
}
