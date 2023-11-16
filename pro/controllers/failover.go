package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	controller "github.com/gravitl/netmaker/controllers"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/mq"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"golang.org/x/exp/slog"
)

type FailOverMeReq struct {
	MyNodeID   string `json:"my_node_id"`
	PeerNodeID string `json:"peer_node_id"`
}

// RelayHandlers - handle Pro Relays
func FailOverHandler(r *mux.Router) {
	r.HandleFunc("/api/v1/host/{hostid}/failoverme", controller.Authorize(true, false, "host", http.HandlerFunc(failOverME))).Methods(http.MethodPost)
}

// swagger:route POST /api/host/failOverME host failOverME
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
	/*
		1. Set On victimNode that needs failedOver to reach - the FailOver and FailedOverBY
		2. On the Node that needs to reach Victim Node, add to failovered Peers
	*/
	var failOverReq FailOverMeReq
	err = json.NewDecoder(r.Body).Decode(&failOverReq)
	if err != nil {
		slog.Error("error decoding request body: ", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	allNodes, err := logic.GetAllNodes()
	if err != nil {
		slog.Error("failed to get all nodes", "error", err)
	}
	peerNode, err := logic.GetNodeByID(failOverReq.PeerNodeID)
	if err != nil {
		slog.Error("failed to get node", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	if peerNode.Network != node.Network {
		slog.Error("node and peer aren't of same network", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if node.IsRelayed {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	failOverNode, err := proLogic.GetFailOverNode(node.Network, allNodes)
	if err != nil {
		slog.Error("auto relay not found", "network", node.Network)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	err = proLogic.SetFailOverCtx(failOverNode, node, peerNode)
	if err != nil {
		slog.Error("failed to create relay:", "id", node.ID.String(),
			"network", node.Network, "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	slog.Info("[auto-relay] created relay on node", "node", node.ID.String(), "network", node.Network)
	go mq.PublishPeerUpdate()
	w.Header().Set("Content-Type", "application/json")
	logic.ReturnSuccessResponse(w, r, "relayed successfully")
}
