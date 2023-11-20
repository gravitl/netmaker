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
	r.HandleFunc("/api/v1/node/{nodeid}/failover", logic.SecurityCheck(true, http.HandlerFunc(createfailOver))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/node/{nodeid}/failover", logic.SecurityCheck(true, http.HandlerFunc(deletefailOver))).Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/node/{network}/failover/reset", logic.SecurityCheck(true, http.HandlerFunc(resetFailOver))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/node/{nodeid}/failover_me", controller.Authorize(true, false, "host", http.HandlerFunc(failOverME))).Methods(http.MethodPost)
}

// swagger:route POST /api/v1/node/failover node createfailOver
//
// Create a relay.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
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
	if _, exists := proLogic.FailOverExists(node.Network); exists {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("failover exists already in the network"), "badrequest"))
		return
	}
	if node.IsRelayed {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("cannot set relayed node as failover"), "badrequest"))
		return
	}
	node.IsFailOver = true
	err = logic.UpsertNode(&node)
	if err != nil {
		slog.Error("failed to upsert node", "node", node.ID.String(), "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	go mq.PublishPeerUpdate()
	w.Header().Set("Content-Type", "application/json")
	logic.ReturnSuccessResponseWithJson(w, r, node, "created failover successfully")
}

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
	go mq.PublishPeerUpdate()
	w.Header().Set("Content-Type", "application/json")
	logic.ReturnSuccessResponse(w, r, "failover has been reset successfully")
}

// swagger:route DELETE /api/v1/node/failover node deletefailOver
//
// Create a relay.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
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
		mq.PublishPeerUpdate()
	}()
	w.Header().Set("Content-Type", "application/json")
	logic.ReturnSuccessResponseWithJson(w, r, node, "deleted failover successfully")
}

// swagger:route POST /api/node/{nodeid}/failOverME node failOver_me
//
// Create a relay.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
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

	failOverNode, exists := proLogic.FailOverExists(node.Network)
	if !exists {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("failover node doesn't exist in the network"), "badrequest"))
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
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("peer not found"), "badrequest"))
		return
	}
	if node.IsRelayed || node.IsFailOver {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("node is relayed or acting as failover"), "badrequest"))
		return
	}
	if peerNode.IsRelayed || peerNode.IsFailOver {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("peer node is relayed or acting as failover"), "badrequest"))
		return
	}

	err = proLogic.SetFailOverCtx(failOverNode, node, peerNode)
	if err != nil {
		slog.Error("failed to create failover", "id", node.ID.String(),
			"network", node.Network, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to create failover: %v", err), "internal"))
		return
	}
	slog.Info("[auto-relay] created relay on node", "node", node.ID.String(), "network", node.Network)
	sendPeerUpdate = true

	if sendPeerUpdate {
		go mq.PublishPeerUpdate()
	}

	w.Header().Set("Content-Type", "application/json")
	logic.ReturnSuccessResponse(w, r, "relayed successfully")
}
