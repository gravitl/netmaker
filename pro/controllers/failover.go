package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	controller "github.com/gravitl/netmaker/controllers"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"golang.org/x/exp/slog"
)

// FailOverHandlers - handlers for FailOver
func FailOverHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/node/{nodeid}/failover", http.HandlerFunc(getfailOver)).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/node/{nodeid}/failover", logic.SecurityCheck(true, http.HandlerFunc(createfailOver))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/node/{nodeid}/failover", logic.SecurityCheck(true, http.HandlerFunc(deletefailOver))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/node/{network}/failover/reset", logic.SecurityCheck(true, http.HandlerFunc(resetFailOver))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/node/{nodeid}/failover_me", controller.Authorize(true, false, "host", http.HandlerFunc(failOverME))).
		Methods(http.MethodPost)
}

// @Summary     Get failover node
// @Router      /api/v1/node/{nodeid}/failover [get]
// @Tags        PRO
// @Param       nodeid path string true "Node ID"
// @Success     200 {object} models.Node
// @Failure     400 {object} models.ErrorResponse
// @Failure     404 {object} models.ErrorResponse
func getfailOver(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	nodeid := params["nodeid"]
	// confirm host exists
	node, err := logic.GetNodeByID(nodeid)
	if err != nil {
		slog.Error("failed to get node:", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	failOverNode, exists := proLogic.FailOverExists(node.Network)
	if !exists {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("failover node not found"), "notfound"),
		)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	logic.ReturnSuccessResponseWithJson(w, r, failOverNode, "get failover node successfully")
}

// @Summary     Create failover node
// @Router      /api/v1/node/{nodeid}/failover [post]
// @Tags        PRO
// @Param       nodeid path string true "Node ID"
// @Success     200 {object} models.Node
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func createfailOver(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	nodeid := params["nodeid"]
	// confirm host exists
	node, err := logic.GetNodeByID(nodeid)
	if err != nil {
		slog.Error("failed to get node:", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	err = proLogic.CreateFailOver(node)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	go mq.PublishPeerUpdate(false)
	w.Header().Set("Content-Type", "application/json")
	logic.ReturnSuccessResponseWithJson(w, r, node, "created failover successfully")
}

// @Summary     Reset failover for a network
// @Router      /api/v1/node/{network}/failover/reset [post]
// @Tags        PRO
// @Param       network path string true "Network ID"
// @Success     200 {object} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func resetFailOver(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	net := params["network"]
	nodes, err := logic.GetNetworkNodes(net)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	for _, node := range nodes {
		if node.FailedOverBy != uuid.Nil {
			node.FailedOverBy = uuid.Nil
			node.FailOverPeers = make(map[string]struct{})
			logic.UpsertNode(&node)
		}
	}
	go mq.PublishPeerUpdate(false)
	w.Header().Set("Content-Type", "application/json")
	logic.ReturnSuccessResponse(w, r, "failover has been reset successfully")
}

// @Summary     Delete failover node
// @Router      /api/v1/node/{nodeid}/failover [delete]
// @Tags        PRO
// @Param       nodeid path string true "Node ID"
// @Success     200 {object} models.Node
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func deletefailOver(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	nodeid := params["nodeid"]
	// confirm host exists
	node, err := logic.GetNodeByID(nodeid)
	if err != nil {
		slog.Error("failed to get node:", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	node.IsFailOver = false
	// Reset FailOvered Peers
	err = logic.UpsertNode(&node)
	if err != nil {
		slog.Error("failed to upsert node", "node", node.ID.String(), "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	go func() {
		proLogic.ResetFailOver(&node)
		mq.PublishPeerUpdate(false)
	}()
	w.Header().Set("Content-Type", "application/json")
	logic.ReturnSuccessResponseWithJson(w, r, node, "deleted failover successfully")
}

// @Summary     Failover me
// @Router      /api/v1/node/{nodeid}/failover_me [post]
// @Tags        PRO
// @Param       nodeid path string true "Node ID"
// @Accept      json
// @Param       body body models.FailOverMeReq true "Failover request"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func failOverME(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	nodeid := params["nodeid"]
	// confirm host exists
	node, err := logic.GetNodeByID(nodeid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to get node:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	host, err := logic.GetHost(node.HostID.String())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	failOverNode, exists := proLogic.FailOverExists(node.Network)
	if !exists {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				fmt.Errorf("req-from: %s, failover node doesn't exist in the network", host.Name),
				"badrequest",
			),
		)
		return
	}
	var failOverReq models.FailOverMeReq
	err = json.NewDecoder(r.Body).Decode(&failOverReq)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	var sendPeerUpdate bool
	peerNode, err := logic.GetNodeByID(failOverReq.NodeID)
	if err != nil {
		slog.Error("peer not found: ", "nodeid", failOverReq.NodeID, "error", err)
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("peer not found"), "badrequest"),
		)
		return
	}
	if node.IsFailOver {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("node is acting as failover"), "badrequest"),
		)
		return
	}
	if node.IsRelayed && node.RelayedBy == peerNode.ID.String() {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("node is relayed by peer node"), "badrequest"),
		)
		return
	}
	if node.IsRelay && peerNode.RelayedBy == node.ID.String() {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("node acting as relay for the peer node"), "badrequest"),
		)
		return
	}
	if node.IsInternetGateway && peerNode.InternetGwID == node.ID.String() {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				errors.New("node acting as internet gw for the peer node"),
				"badrequest",
			),
		)
		return
	}
	if node.InternetGwID != "" && node.InternetGwID == peerNode.ID.String() {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				errors.New("node using a internet gw by the peer node"),
				"badrequest",
			),
		)
		return
	}

	err = proLogic.SetFailOverCtx(failOverNode, node, peerNode)
	if err != nil {
		slog.Error("failed to create failover", "id", node.ID.String(),
			"network", node.Network, "error", err)
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(fmt.Errorf("failed to create failover: %v", err), "internal"),
		)
		return
	}
	slog.Info(
		"[auto-relay] created relay on node",
		"node",
		node.ID.String(),
		"network",
		node.Network,
	)
	sendPeerUpdate = true

	if sendPeerUpdate {
		go mq.PublishPeerUpdate(false)
	}

	w.Header().Set("Content-Type", "application/json")
	logic.ReturnSuccessResponse(w, r, "relayed successfully")
}
