package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
)

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
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				errors.New("only linux nodes can be made internet gws"),
				"badrequest",
			),
		)
		return
	}
	err = logic.ValidateInetGwReq(node, request, false)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logic.SetInternetGw(&node, request)
	if servercfg.IsPro {
		if _, exists := logic.FailOverExists(node.Network); exists {
			go func() {
				logic.ResetFailedOverPeer(&node)
				mq.PublishPeerUpdate(false)
			}()
		}
		go func() {
			logic.ResetAutoRelayedPeer(&node)
			mq.PublishPeerUpdate(false)
		}()

	}
	if node.IsGw && node.IngressDNS == "" {
		node.IngressDNS = "1.1.1.1"
	}
	err = logic.UpsertNode(&node)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	apiNode := node.ConvertToAPINode()
	logger.Log(
		1,
		r.Header.Get("user"),
		"created ingress gateway on node",
		nodeid,
		"on network",
		netid,
	)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
	go mq.PublishPeerUpdate(false)
}

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
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("node is not a internet gw"), "badrequest"),
		)
		return
	}
	err = logic.ValidateInetGwReq(node, request, true)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logic.UnsetInternetGw(&node)
	logic.SetInternetGw(&node, request)
	err = logic.UpsertNode(&node)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	apiNode := node.ConvertToAPINode()
	logger.Log(
		1,
		r.Header.Get("user"),
		"created ingress gateway on node",
		nodeid,
		"on network",
		netid,
	)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
	go mq.PublishPeerUpdate(false)
}

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

	logic.UnsetInternetGw(&node)
	err = logic.UpsertNode(&node)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	apiNode := node.ConvertToAPINode()
	logger.Log(
		1,
		r.Header.Get("user"),
		"created ingress gateway on node",
		nodeid,
		"on network",
		netid,
	)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
	go mq.PublishPeerUpdate(false)
}
