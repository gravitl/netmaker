package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
)

func gwHandlers(r *mux.Router) {
	r.HandleFunc("/api/nodes/{network}/{nodeid}/gateway", logic.SecurityCheck(true, checkFreeTierLimits(limitChoiceIngress, http.HandlerFunc(createGateway)))).Methods(http.MethodPost)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/gateway", logic.SecurityCheck(true, http.HandlerFunc(deleteGateway))).Methods(http.MethodDelete)
	// old relay handlers
	r.HandleFunc("/api/nodes/{network}/{nodeid}/createrelay", logic.SecurityCheck(true, http.HandlerFunc(createGateway))).Methods(http.MethodPost)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/deleterelay", logic.SecurityCheck(true, http.HandlerFunc(deleteGateway))).Methods(http.MethodDelete)
}

// @Summary     Create a gateway
// @Router      /api/nodes/{network}/{nodeid}/gateway [post]
// @Tags        Nodes
// @Security    oauth2
// @Success     200 {object} models.ApiNode
// @Failure     500 {object} models.ErrorResponse
func createGateway(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	nodeid := params["nodeid"]
	netid := params["network"]
	node, err := logic.ValidateParams(nodeid, netid)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	var req models.CreateGwReq
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	node, err = logic.CreateIngressGateway(netid, nodeid, req.IngressRequest)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to create gateway on node [%s] on network [%s]: %v",
				nodeid, netid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	req.RelayRequest.NetID = netid
	req.RelayRequest.NodeID = nodeid
	_, relayNode, err := logic.CreateRelay(req.RelayRequest)
	if err != nil {
		logger.Log(
			0,
			r.Header.Get("user"),
			fmt.Sprintf(
				"failed to create relay on node [%s] on network [%s]: %v",
				req.RelayRequest.NodeID,
				req.RelayRequest.NetID,
				err,
			),
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	for _, relayedNodeID := range relayNode.RelayedNodes {
		relayedNode, err := logic.GetNodeByID(relayedNodeID)
		if err == nil {
			if relayedNode.FailedOverBy != uuid.Nil {
				go logic.ResetFailedOverPeer(&relayedNode)
			}

		}
	}
	logger.Log(
		1,
		r.Header.Get("user"),
		"created gw node",
		req.RelayRequest.NodeID,
		"on network",
		req.RelayRequest.NetID,
	)
	apiNode := relayNode.ConvertToAPINode()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
	go func() {
		if err := mq.NodeUpdate(&node); err != nil {
			slog.Error("error publishing node update to node", "node", node.ID, "error", err)
		}
		mq.PublishPeerUpdate(false)
	}()

}

// @Summary     Delete a gateway
// @Router      /api/nodes/{network}/{nodeid}/gateway [delete]
// @Tags        Nodes
// @Security    oauth2
// @Success     200 {object} models.ApiNode
// @Failure     500 {object} models.ErrorResponse
func deleteGateway(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	nodeid := params["nodeid"]
	netid := params["network"]
	node, err := logic.ValidateParams(nodeid, netid)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	node, removedClients, err := logic.DeleteIngressGateway(nodeid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to delete ingress gateway on node [%s] on network [%s]: %v",
				nodeid, netid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	updateNodes, node, err := logic.DeleteRelay(netid, nodeid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	node, err = logic.GetNodeByID(node.ID.String())
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to get node", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	node.IsGw = false
	logic.UpsertNode(&node)
	logger.Log(1, r.Header.Get("user"), "deleted gw", nodeid, "on network", netid)

	go func() {
		host, err := logic.GetHost(node.HostID.String())
		if err == nil {
			allNodes, err := logic.GetAllNodes()
			if err != nil {
				return
			}

			for _, relayedNode := range updateNodes {
				err = mq.NodeUpdate(&relayedNode)
				if err != nil {
					logger.Log(
						1,
						"relayed node update ",
						relayedNode.ID.String(),
						"on network",
						relayedNode.Network,
						": ",
						err.Error(),
					)

				}
				h, err := logic.GetHost(relayedNode.HostID.String())
				if err == nil {
					if h.OS == models.OS_Types.IoT {
						nodes, err := logic.GetAllNodes()
						if err != nil {
							return
						}
						node.IsRelay = true // for iot update to recognise that it has to delete relay peer
						if err = mq.PublishSingleHostPeerUpdate(h, nodes, &node, nil, false, nil); err != nil {
							logger.Log(1, "failed to publish peer update to host", h.ID.String(), ": ", err.Error())
						}
					}
				}
			}
			if len(removedClients) > 0 {
				if err := mq.PublishSingleHostPeerUpdate(host, allNodes, nil, removedClients[:], false, nil); err != nil {
					slog.Error("publishSingleHostUpdate", "host", host.Name, "error", err)
				}
			}
			mq.PublishPeerUpdate(false)
			if err := mq.NodeUpdate(&node); err != nil {
				slog.Error(
					"error publishing node update to node",
					"node",
					node.ID,
					"error",
					err,
				)
			}
			if servercfg.IsDNSMode() {
				logic.SetDNS()
			}

		}

	}()

	apiNode := node.ConvertToAPINode()
	logger.Log(1, r.Header.Get("user"), "deleted ingress gateway", nodeid)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
}
