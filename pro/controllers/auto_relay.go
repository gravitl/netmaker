package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	controller "github.com/gravitl/netmaker/controllers"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
	"golang.org/x/exp/slog"
)

// AutoRelayHandlers - handlers for AutoRelay
func AutoRelayHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/node/{nodeid}/auto_relay", controller.Authorize(true, false, "host", http.HandlerFunc(getAutoRelayGws))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/node/{nodeid}/auto_relay", logic.SecurityCheck(true, http.HandlerFunc(setAutoRelay))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/node/{nodeid}/auto_relay", logic.SecurityCheck(true, http.HandlerFunc(unsetAutoRelay))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/node/{network}/auto_relay/reset", logic.SecurityCheck(true, http.HandlerFunc(resetAutoRelayGw))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/node/{nodeid}/auto_relay_me", controller.Authorize(true, false, "host", http.HandlerFunc(autoRelayME))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/node/{nodeid}/auto_relay_check", controller.Authorize(true, false, "host", http.HandlerFunc(checkautoRelayCtx))).
		Methods(http.MethodGet)
}

// @Summary     Get auto relay nodes
// @Router      /api/v1/node/{nodeid}/auto_relay [get]
// @Tags        PRO
// @Param       nodeid path string true "Node ID"
// @Success     200 {object} models.Node
// @Failure     400 {object} models.ErrorResponse
// @Failure     404 {object} models.ErrorResponse
func getAutoRelayGws(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	nodeid := params["nodeid"]
	// confirm host exists
	node, err := logic.GetNodeByID(nodeid)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	nodes, _ := logic.GetNetworkNodes(node.Network)
	for _, node := range nodes {
		if node.IsAutoRelay {

		}
	}
	autoRelayNode, exists := proLogic.DoesAutoRelayExist(node.Network)
	if !exists {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("autorelay node not found"), "notfound"),
		)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	logic.ReturnSuccessResponseWithJson(w, r, autoRelayNode, "get autorelay node successfully")
}

// @Summary     Create AutoRelay node
// @Router      /api/v1/node/{nodeid}/auto_relay [post]
// @Tags        PRO
// @Param       nodeid path string true "Node ID"
// @Success     200 {object} models.Node
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func setAutoRelay(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	nodeid := params["nodeid"]
	// confirm host exists
	node, err := logic.GetNodeByID(nodeid)
	if err != nil {
		slog.Error("failed to get node:", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	err = proLogic.CreateAutoRelay(node)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	go mq.PublishPeerUpdate(false)
	w.Header().Set("Content-Type", "application/json")
	logic.ReturnSuccessResponseWithJson(w, r, node, "created autorelay successfully")
}

// @Summary     Reset AutoRelay for a network
// @Router      /api/v1/node/{network}/auto_relay/reset [post]
// @Tags        PRO
// @Param       network path string true "Network ID"
// @Success     200 {object} models.SuccessResponse
// @Failure     500 {object} models.ErrorResponse
func resetAutoRelayGw(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	net := params["network"]
	nodes, err := logic.GetNetworkNodes(net)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	for _, node := range nodes {
		if node.AutoRelayedBy != uuid.Nil {
			node.AutoRelayedBy = uuid.Nil
			if node.Mutex != nil {
				node.Mutex.Lock()
			}
			node.AutoRelayedPeers = make(map[string]struct{})
			if node.Mutex != nil {
				node.Mutex.Unlock()
			}
			logic.UpsertNode(&node)
		}
	}
	go mq.PublishPeerUpdate(false)
	w.Header().Set("Content-Type", "application/json")
	logic.ReturnSuccessResponse(w, r, "autorelay has been reset successfully")
}

// @Summary     Delete autorelay node
// @Router      /api/v1/node/{nodeid}/auto_relay [delete]
// @Tags        PRO
// @Param       nodeid path string true "Node ID"
// @Success     200 {object} models.Node
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func unsetAutoRelay(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	nodeid := params["nodeid"]
	// confirm host exists
	node, err := logic.GetNodeByID(nodeid)
	if err != nil {
		slog.Error("failed to get node:", "error", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	node.IsAutoRelay = false
	// Reset AutoRelayed Peers
	err = logic.UpsertNode(&node)
	if err != nil {
		slog.Error("failed to upsert node", "node", node.ID.String(), "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	proLogic.RemoveAutoRelayFromCache(node.Network)
	go func() {
		proLogic.ResetAutoRelay(&node)
		mq.PublishPeerUpdate(false)
	}()
	w.Header().Set("Content-Type", "application/json")
	logic.ReturnSuccessResponseWithJson(w, r, node, "deleted autorelay successfully")
}

// @Summary     AutoRelay me
// @Router      /api/v1/node/{nodeid}/auto_relay_me [post]
// @Tags        PRO
// @Param       nodeid path string true "Node ID"
// @Accept      json
// @Param       body body models.AutoRelayMeReq true "AutoRelay request"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func autoRelayME(w http.ResponseWriter, r *http.Request) {
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

	autoRelayNode, exists := proLogic.DoesAutoRelayExist(node.Network)
	if !exists {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				fmt.Errorf("req-from: %s, autorelay node doesn't exist in the network", host.Name),
				"badrequest",
			),
		)
		return
	}
	var autoRelayReq models.AutoRelayMeReq
	err = json.NewDecoder(r.Body).Decode(&autoRelayReq)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	var sendPeerUpdate bool
	peerNode, err := logic.GetNodeByID(autoRelayReq.NodeID)
	if err != nil {
		slog.Error("peer not found: ", "nodeid", autoRelayReq.NodeID, "error", err)
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("peer not found"), "badrequest"),
		)
		return
	}
	eli, _ := (&schema.Egress{Network: node.Network}).ListByNetwork(db.WithContext(context.TODO()))
	acls, _ := logic.ListAclsByNetwork(models.NetworkID(node.Network))
	logic.GetNodeEgressInfo(&node, eli, acls)
	logic.GetNodeEgressInfo(&peerNode, eli, acls)
	logic.GetNodeEgressInfo(&autoRelayNode, eli, acls)
	if peerNode.IsAutoRelay {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("peer is acting as autorelay"), "badrequest"),
		)
		return
	}
	if node.IsAutoRelay {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("node is acting as autorelay"), "badrequest"),
		)
		return
	}
	if peerNode.IsAutoRelay {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("peer is acting as autorelay"), "badrequest"),
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
	if (node.InternetGwID != "" && autoRelayNode.IsInternetGateway && node.InternetGwID != autoRelayNode.ID.String()) ||
		(peerNode.InternetGwID != "" && autoRelayNode.IsInternetGateway && peerNode.InternetGwID != autoRelayNode.ID.String()) {
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
	err = proLogic.SetAutoRelayCtx(autoRelayNode, node, peerNode)
	if err != nil {
		slog.Debug("failed to create autorelay", "id", node.ID.String(),
			"network", node.Network, "error", err)
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(fmt.Errorf("failed to create autorelay: %v", err), "internal"),
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

// @Summary     checkautoRelayCtx
// @Router      /api/v1/node/{nodeid}/auto_relay_check [get]
// @Tags        PRO
// @Param       nodeid path string true "Node ID"
// @Accept      json
// @Param       body body models.AutoRelayMeReq true "autorelay request"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func checkautoRelayCtx(w http.ResponseWriter, r *http.Request) {
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

	autoRelayNode, exists := proLogic.DoesAutoRelayExist(node.Network)
	if !exists {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				fmt.Errorf("req-from: %s, autorelay node doesn't exist in the network", host.Name),
				"badrequest",
			),
		)
		return
	}
	var autoRelayReq models.AutoRelayMeReq
	err = json.NewDecoder(r.Body).Decode(&autoRelayReq)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	peerNode, err := logic.GetNodeByID(autoRelayReq.NodeID)
	if err != nil {
		slog.Error("peer not found: ", "nodeid", autoRelayReq.NodeID, "error", err)
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("peer not found"), "badrequest"),
		)
		return
	}
	eli, _ := (&schema.Egress{Network: node.Network}).ListByNetwork(db.WithContext(context.TODO()))
	acls, _ := logic.ListAclsByNetwork(models.NetworkID(node.Network))
	logic.GetNodeEgressInfo(&node, eli, acls)
	logic.GetNodeEgressInfo(&peerNode, eli, acls)
	logic.GetNodeEgressInfo(&autoRelayNode, eli, acls)
	if peerNode.IsAutoRelay {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("peer is acting as autorelay"), "badrequest"),
		)
		return
	}
	if node.IsAutoRelay {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("node is acting as autorelay"), "badrequest"),
		)
		return
	}
	if peerNode.IsAutoRelay {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(errors.New("peer is acting as autorelay"), "badrequest"),
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
	if (node.InternetGwID != "" && autoRelayNode.IsInternetGateway && node.InternetGwID != autoRelayNode.ID.String()) ||
		(peerNode.InternetGwID != "" && autoRelayNode.IsInternetGateway && peerNode.InternetGwID != autoRelayNode.ID.String()) {
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
	if ok := logic.IsPeerAllowed(node, peerNode, true); !ok {
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				errors.New("peers are not allowed to communicate"),
				"badrequest",
			),
		)
		return
	}

	err = proLogic.CheckAutoRelayCtx(autoRelayNode, node, peerNode)
	if err != nil {
		slog.Error("autorelay ctx cannot be set ", "error", err)
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(fmt.Errorf("autorelay ctx cannot be set: %v", err), "internal"),
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	logic.ReturnSuccessResponse(w, r, "autorelay can be set")
}
