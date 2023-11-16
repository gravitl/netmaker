package controllers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	controller "github.com/gravitl/netmaker/controllers"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"golang.org/x/exp/slog"
)

type FailOverMeReq struct {
	PeerPubKey string `json:"peer_pub_key"`
}

// FailOverHandlers - handlers for FailOver
func FailOverHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/node/{nodeid}/failover", logic.SecurityCheck(true, http.HandlerFunc(createfailOver))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/node/{nodeid}/failover", logic.SecurityCheck(true, http.HandlerFunc(deletefailOver))).Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/host/{hostid}/failover_me", controller.Authorize(true, false, "host", http.HandlerFunc(failOverME))).Methods(http.MethodPost)
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
	if proLogic.FailOverExists(node.Network) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("failover exists already in the network"), "badrequest"))
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
	logic.ReturnSuccessResponse(w, r, "relayed successfully")
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
		proLogic.ResetFailOveredPeers(&node)
		mq.PublishPeerUpdate()
	}()
	w.Header().Set("Content-Type", "application/json")
	logic.ReturnSuccessResponse(w, r, "relayed successfully")
}

// swagger:route POST /api/host/failOverME host failOver_me
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
	hostid := params["hostid"]
	// confirm host exists
	host, err := logic.GetHost(hostid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to get host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	var failOverReq FailOverMeReq
	err = json.NewDecoder(r.Body).Decode(&failOverReq)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	var sendPeerUpdate bool
	allNodes, err := logic.GetAllNodes()
	if err != nil {
		slog.Error("failed to get all nodes", "error", err)
	}
	peerHost, err := logic.GetHostByPubKey(failOverReq.PeerPubKey)
	if err != nil {
		slog.Error("peer not found: ", "publickey", failOverReq.PeerPubKey, "error", err)
		return
	}
	peerNodeMap := make(map[string]models.Node)
	for _, nodeID := range peerHost.Nodes {
		node, err := logic.GetNodeByID(nodeID)
		if err != nil {
			continue
		}
		peerNodeMap[node.Network] = node
	}
	for _, nodeID := range host.Nodes {
		node, err := logic.GetNodeByID(nodeID)
		if err != nil {
			slog.Error("couldn't find node", "id", nodeID, "error", err)
			continue
		}
		if node.IsRelayed {
			continue
		}
		peerNode, ok := peerNodeMap[node.Network]
		if !ok {
			continue
		}
		if peerNode.IsRelayed {
			continue
		}
		// get auto relay Host in this network
		failOverNode, err := proLogic.GetFailOverNode(node.Network, allNodes)
		if err != nil {
			slog.Error("auto relay not found", "network", node.Network)
			continue
		}
		err = proLogic.SetFailOverCtx(failOverNode, node, peerNode)
		if err != nil {
			slog.Error("failed to create relay:", "id", node.ID.String(),
				"network", node.Network, "error", err)
			continue
		}
		slog.Info("[auto-relay] created relay on node", "node", node.ID.String(), "network", node.Network)
		sendPeerUpdate = true
	}

	if sendPeerUpdate {
		go mq.PublishPeerUpdate()
	}

	w.Header().Set("Content-Type", "application/json")
	logic.ReturnSuccessResponse(w, r, "relayed successfully")
}
