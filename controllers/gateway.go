package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"

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
	r.HandleFunc("/api/nodes/{network}/{nodeid}/gateway/assign", logic.SecurityCheck(true, http.HandlerFunc(assignGw))).Methods(http.MethodPost)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/gateway/unassign", logic.SecurityCheck(true, http.HandlerFunc(unassignGw))).Methods(http.MethodPost)
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
	host, err := logic.GetHost(node.HostID.String())
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
	if req.IsInternetGateway && len(req.InetNodeClientIDs) > 0 {
		err = logic.ValidateInetGwReq(node, req.InetNodeReq, false)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
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
	if req.IsInternetGateway {
		if host.DNS != "yes" {
			host.DNS = "yes"
			logic.UpsertHost(host)
		}
	}
	for _, relayedNodeID := range relayNode.RelayedNodes {
		relayedNode, err := logic.GetNodeByID(relayedNodeID)
		if err == nil {
			if relayedNode.FailedOverBy != uuid.Nil {
				go logic.ResetFailedOverPeer(&relayedNode)
			}
			if len(relayedNode.AutoRelayedPeers) > 0 {
				go logic.ResetAutoRelayedPeer(&relayedNode)
			}

		}
	}
	if len(req.InetNodeClientIDs) > 0 {
		logic.SetInternetGw(&node, req.InetNodeReq)
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
		logic.UpsertNode(&node)
	}

	logger.Log(
		1,
		r.Header.Get("user"),
		"created gw node",
		req.RelayRequest.NodeID,
		"on network",
		req.RelayRequest.NetID,
	)
	logic.GetNodeStatus(&relayNode, false)
	apiNode := relayNode.ConvertToAPINode()
	logic.LogEvent(&models.Event{
		Action: models.Create,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: models.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   node.ID.String(),
			Name: host.Name,
			Type: models.GatewaySub,
		},
		Origin: models.Dashboard,
	})
	host.IsStaticPort = true
	logic.UpsertHost(host)
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
	host, err := logic.GetHost(node.HostID.String())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logic.UnsetInternetGw(&node)
	node.IsGw = false
	if node.IsAutoRelay {
		logic.ResetAutoRelay(&node)
	}
	node.IsAutoRelay = false
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
	logic.LogEvent(&models.Event{
		Action: models.Delete,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: models.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   node.ID.String(),
			Name: host.Name,
			Type: models.GatewaySub,
		},
		Origin: models.Dashboard,
		Diff: models.Diff{
			Old: node,
			New: node,
		},
	})
	logic.GetNodeStatus(&node, false)
	apiNode := node.ConvertToAPINode()
	logger.Log(1, r.Header.Get("user"), "deleted ingress gateway", nodeid)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
}

// @Summary     Assign a node to a gateway
// @Router      /api/nodes/{network}/{nodeid}/gateway/assign [post]
// @Tags        Nodes
// @Security    oauth2
// @Param       network path string true "Network ID"
// @Param       nodeid path string true "Client node ID to assign to gateway"
// @Param       gw_id query string true "Gateway node ID"
// @Param       auto_assign_gw query bool false "Enable auto-assign gateway (Pro only)"
// @Success     200 {object} models.ApiNode
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func assignGw(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	nodeid := params["nodeid"]
	netid := params["network"]
	gwid := r.URL.Query().Get("gw_id")
	autoAssignGw := r.URL.Query().Get("auto_assign_gw") == "true"
	// Validate client node
	node, err := logic.ValidateParams(nodeid, netid)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if !servercfg.IsPro {
		autoAssignGw = false
	}
	if autoAssignGw {
		if node.FailedOverBy != uuid.Nil {
			go logic.ResetFailedOverPeer(&node)
		}
		if len(node.AutoRelayedPeers) > 0 {
			go logic.ResetAutoRelayedPeer(&node)
		}
		if node.RelayedBy != "" {
			gatewayNode, err := logic.GetNodeByID(node.RelayedBy)
			if err == nil {
				// check if gw gateway Node has the relayed Node
				if !slices.Contains(gatewayNode.RelayedNodes, node.ID.String()) {
					gatewayNode.RelayedNodes = append(gatewayNode.RelayedNodes, node.ID.String())
				}
				newNodes := gatewayNode.RelayedNodes
				newNodes = logic.RemoveAllFromSlice(newNodes, node.ID.String())
				logic.UpdateRelayNodes(gatewayNode.ID.String(), gatewayNode.RelayedNodes, newNodes)
				// Unassign client nodes (set their InternetGwID to empty)
				if node.InternetGwID != "" {
					node.InternetGwID = ""
					gatewayNode.InetNodeReq.InetNodeClientIDs = logic.RemoveAllFromSlice(gatewayNode.InetNodeReq.InetNodeClientIDs, node.ID.String())
					logic.UpsertNode(&gatewayNode)
				}
			} else {
				node.RelayedBy = ""
				node.InternetGwID = ""
			}
			node, _ = logic.GetNodeByID(node.ID.String())
		}
		node.AutoAssignGateway = true
		logic.UpsertNode(&node)
		logic.GetNodeStatus(&node, false)
		go func() {
			if err := mq.NodeUpdate(&node); err != nil {
				slog.Error("error publishing node update to node", "node", node.ID, "error", err)
			}
			mq.PublishPeerUpdate(false)
		}()
		logic.ReturnSuccessResponseWithJson(w, r, node.ConvertToAPINode(), "auto assigned gateway")
		return
	}
	// Validate gateway node
	gatewayNode, err := logic.ValidateParams(gwid, netid)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	// Check if node is a gateway
	if !gatewayNode.IsGw {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("node %s is not a gateway", nodeid), "badrequest"))
		return
	}

	if node.FailedOverBy != uuid.Nil {
		go logic.ResetFailedOverPeer(&node)
	}
	if len(node.AutoRelayedPeers) > 0 {
		go logic.ResetAutoRelayedPeer(&node)
	}
	newNodes := []string{node.ID.String()}
	newNodes = append(newNodes, gatewayNode.RelayedNodes...)
	newNodes = logic.UniqueStrings(newNodes)
	logic.UpdateRelayNodes(gatewayNode.ID.String(), gatewayNode.RelayedNodes, newNodes)

	host, err := logic.GetHost(node.HostID.String())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	logger.Log(1, r.Header.Get("user"),
		fmt.Sprintf("assigned nodes to gateway [%s] on network [%s]",
			nodeid, netid))

	logic.LogEvent(&models.Event{
		Action: models.GatewayAssign,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: models.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   node.ID.String(),
			Name: host.Name,
			Type: models.GatewaySub,
		},
		Origin: models.Dashboard,
	})

	logic.GetNodeStatus(&node, false)
	apiNode := node.ConvertToAPINode()

	go func() {
		if err := mq.NodeUpdate(&node); err != nil {
			slog.Error("error publishing node update to node", "node", node.ID, "error", err)
		}
		mq.PublishPeerUpdate(false)
	}()

	logic.ReturnSuccessResponseWithJson(w, r, apiNode, "assigned gateway")
}

// @Summary     Unassign client nodes from a gateway
// @Router      /api/nodes/{network}/{nodeid}/gateway/unassign [post]
// @Tags        Nodes
// @Security    oauth2
// @Param       body body models.InetNodeReq true "Internet gateway request with client node IDs to unassign"
// @Success     200 {object} models.ApiNode
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func unassignGw(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	nodeid := params["nodeid"]
	netid := params["network"]
	// Validate gateway node
	node, err := logic.ValidateParams(nodeid, netid)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	host, err := logic.GetHost(node.HostID.String())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	gwid := node.RelayedBy
	if node.AutoAssignGateway && gwid == "" {
		node.AutoAssignGateway = false
		logic.UpsertNode(&node)
		go func() {
			if err := mq.NodeUpdate(&node); err != nil {
				slog.Error("error publishing node update to node", "node", node.ID, "error", err)
			}
			mq.PublishPeerUpdate(false)
		}()
		logic.ReturnSuccessResponseWithJson(w, r, node.ConvertToAPINode(), "unassigned gateway")
		return
	}

	// Validate gateway node
	gatewayNode, err := logic.ValidateParams(gwid, netid)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	// Unassign client nodes (set their InternetGwID to empty)
	if node.InternetGwID != "" {
		node.InternetGwID = ""
		gatewayNode.InetNodeReq.InetNodeClientIDs = logic.RemoveAllFromSlice(gatewayNode.InetNodeReq.InetNodeClientIDs, node.ID.String())
	}
	node.AutoAssignGateway = false
	logic.UpsertNode(&node)
	logic.UpsertNode(&gatewayNode)
	// Unset Relayed node
	// check if gw gateway Node has the relayed Node
	if !slices.Contains(gatewayNode.RelayedNodes, node.ID.String()) {
		gatewayNode.RelayedNodes = append(gatewayNode.RelayedNodes, node.ID.String())
	}
	newNodes := gatewayNode.RelayedNodes
	newNodes = logic.RemoveAllFromSlice(newNodes, node.ID.String())
	logic.UpdateRelayNodes(gatewayNode.ID.String(), gatewayNode.RelayedNodes, newNodes)
	node, _ = logic.GetNodeByID(node.ID.String())
	logger.Log(1, r.Header.Get("user"),
		fmt.Sprintf("unassigned client nodes from gateway [%s] on network [%s]",
			nodeid, netid))

	logic.LogEvent(&models.Event{
		Action: models.GatewayUnAssign,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: models.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   node.ID.String(),
			Name: host.Name,
			Type: models.GatewaySub,
		},
		Origin: models.Dashboard,
	})

	logic.GetNodeStatus(&node, false)

	go func() {
		if err := mq.NodeUpdate(&node); err != nil {
			slog.Error("error publishing node update to node", "node", node.ID, "error", err)
		}
		mq.PublishPeerUpdate(false)
	}()
	logic.ReturnSuccessResponseWithJson(w, r, node.ConvertToAPINode(), "unassigned gateway")
}
