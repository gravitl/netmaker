package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/middleware"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/orchestrator"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func gwHandlers(r *mux.Router) {
	r.HandleFunc("/api/nodes/{network}/{nodeid}/gateway", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(createGateway)))).Methods(http.MethodPost)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/gateway", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(deleteGateway)))).Methods(http.MethodDelete)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/gateway/assign", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(assignGw)))).Methods(http.MethodPost)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/gateway/unassign", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(unassignGw)))).Methods(http.MethodPost)
	// old relay handlers
	r.HandleFunc("/api/nodes/{network}/{nodeid}/createrelay", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(createGateway)))).Methods(http.MethodPost)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/deleterelay", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(deleteGateway)))).Methods(http.MethodDelete)
}

// @Summary     Create a gateway
// @Router      /api/nodes/{network}/{nodeid}/gateway [post]
// @Tags        Gateways
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       network path string true "Network ID"
// @Param       nodeid path string true "Node ID"
// @Param       body body models.CreateGwReq true "Gateway request"
// @Success     200 {object} models.ApiNode
// @Failure     500 {object} models.ErrorResponse
func createGateway(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	nodeID := params["nodeid"]
	networkName := params["network"]

	node := &schema.Node{
		ID: nodeID,
	}
	err := node.Get(r.Context(), dbtypes.WithAllPreloads())
	if err != nil {
		errType := logic.Internal
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errType = logic.BadReq
		}

		err = fmt.Errorf("failed to create gateway on node (%s) in network (%s): error fetching node (%s): %v", nodeID, networkName, nodeID, err)
		logger.Log(0, r.Header.Get("user"), err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, errType))
		return
	}

	if node.Network.Name != networkName {
		err = fmt.Errorf("failed to create gateway on node (%s) in network (%s): node (%s) not in network (%s)", nodeID, networkName, nodeID, networkName)
		logger.Log(0, r.Header.Get("user"), err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	var req models.CreateGwReq
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logger.Log(
			0,
			r.Header.Get("user"),
			fmt.Sprintf("failed to create gateway on node (%s) in network (%s): error parsing request: %v", nodeID, networkName, err),
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	var options []orchestrator.Option
	if len(req.RelayedNodes) > 0 {
		options = append(options, orchestrator.WithRelayedClients(req.RelayedNodes))
	}

	if req.IsInternetGateway {
		options = append(options, orchestrator.WithInternetGateway(req.InetNodeClientIDs))
	}

	err = orchestrator.GetRepository().NodeOrchestrator().ValidateCreateGateway(r.Context(), node, options...)
	if err != nil {
		logger.Log(
			0,
			r.Header.Get("user"),
			fmt.Sprintf("failed to create gateway on node (%s) in network (%s): error validating request: %v", nodeID, networkName, err),
		)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	err = orchestrator.GetRepository().NodeOrchestrator().CreateGateway(r.Context(), node, options...)
	if err != nil {
		err = fmt.Errorf("failed to create gateway on node (%s) in network (%s): error creating gateway: %v", nodeID, networkName, err)
		logger.Log(0, r.Header.Get("user"), err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	logger.Log(
		0,
		r.Header.Get("user"),
		fmt.Sprintf("created gw node %s on network %s", nodeID, networkName),
	)

	node.Status = logic.GetNodeCheckInStatus(node)
	apiNode := logic.ConvertSchemaNodeToApiNode(node)

	logic.LogEvent(&models.Event{
		Action: schema.Create,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   node.ID,
			Name: node.Host.Name,
			Type: schema.GatewaySub,
		},
		Origin: schema.Dashboard,
	})

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(apiNode)
}

// @Summary     Delete a gateway
// @Router      /api/nodes/{network}/{nodeid}/gateway [delete]
// @Tags        Gateways
// @Security    oauth
// @Produce     json
// @Param       network path string true "Network ID"
// @Param       nodeid path string true "Node ID"
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
	host := &schema.Host{
		ID: node.HostID,
	}
	err = host.Get(r.Context())
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

	// TODO: currently only for cleanup, but later this should be used as the main function.
	_node := &schema.Node{
		ID: node.ID.String(),
	}
	err = _node.Get(r.Context())
	if err == nil {
		_node.IsGateway = false
		_node.IsInternetGateway = false
		_node.IsAutoRelay = "no"
		_node.RelayedClients = make(datatypes.JSONMap)
		_node.RelayedIGWClients = make(datatypes.JSONMap)
		_node.AdditionalGatewayEndpoints = make(datatypes.JSONSlice[string], 0)
		_ = _node.ResetGateway(r.Context())
	}

	logger.Log(1, r.Header.Get("user"), "deleted gw", nodeid, "on network", netid)

	go func() {
		host := &schema.Host{
			ID: node.HostID,
		}
		err = host.Get(db.WithContext(context.TODO()))
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
				h := &schema.Host{
					ID: relayedNode.HostID,
				}
				err = h.Get(db.WithContext(context.TODO()))
				if err == nil {
					if h.OS == models.OS_Types.IoT {
						nodes, err := logic.GetAllNodes()
						if err != nil {
							return
						}
						node.IsRelay = true // for iot update to recognise that it has to delete relay peer
						if err = mq.PublishSingleHostPeerUpdate(h, nodes, nil, &node, nil, false, nil); err != nil {
							logger.Log(1, "failed to publish peer update to host", h.ID.String(), ": ", err.Error())
						}
					}
				}
			}
			if len(removedClients) > 0 {
				if err := mq.PublishSingleHostPeerUpdate(host, allNodes, nil, nil, removedClients[:], false, nil); err != nil {
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
		}

		logic.RemoveNodeFromEnrollmentKeys(&node)
	}()
	logic.LogEvent(&models.Event{
		Action: schema.Delete,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   node.ID.String(),
			Name: host.Name,
			Type: schema.GatewaySub,
		},
		Origin: schema.Dashboard,
		Diff: models.Diff{
			Old: node,
			New: node,
		},
	})
	apiNode := node.ConvertToAPINode()
	logger.Log(1, r.Header.Get("user"), "deleted ingress gateway", nodeid)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
}

// @Summary     Assign a node to a gateway
// @Router      /api/nodes/{network}/{nodeid}/gateway/assign [post]
// @Tags        Gateways
// @Security    oauth
// @Produce     json
// @Param       network path string true "Network ID"
// @Param       nodeid path string true "Client node ID to assign to gateway"
// @Param       gw_id query string true "Gateway node ID"
// @Param       auto_assign_gw query bool false "Enable auto-assign gateway (Pro only)"
// @Success     200 {object} models.ApiNode
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func assignGw(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	nodeID := params["nodeid"]
	networkName := params["network"]
	gatewayID := r.URL.Query().Get("gw_id")
	autoAssignGw := r.URL.Query().Get("auto_assign_gw") == "true"

	node := &schema.Node{
		ID: nodeID,
	}
	err := node.Get(r.Context(), dbtypes.WithAllPreloads())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	if node.Network.Name != networkName {
		err = fmt.Errorf("network url param does not match node network")
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	if !servercfg.IsPro {
		autoAssignGw = false
	}

	if autoAssignGw {
		if node.RelayedByNodeID != nil {
			if node.IsIGWClient {
				err = errors.New("node is configured to route all traffic via an internet gateway; auto-assign gateway is not allowed")
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
				return
			}

			gateway := &schema.Node{
				ID: *node.RelayedByNodeID,
			}
			err = gateway.Get(r.Context())
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				err = fmt.Errorf("failed to enable auto assign gateway for node (%s): error getting current gateway (%s): %v", node.ID, gateway.ID, err)
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
				return
			}

			node.RelayedByNodeID = nil
			node.IsIGWClient = false
			err = node.UnassignGateway(r.Context())
			if err != nil {
				err = fmt.Errorf("failed to enable auto assign gateway for node (%s): error unassigning current gateway(%s): %v", node.ID, gateway.ID, err)
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
				return
			}
		}

		node.AutoAssignGateway = true
		err = node.SetAutoAssignGateway(r.Context())
		if err != nil {
			err = fmt.Errorf("failed to enable auto assign gateway for node (%s): %v", node.ID, err)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
			return
		}

		modelsNode := logic.ConvertSchemaNodeToModelsNode(node)

		go func() {
			if len(node.AutoRelayedPeers.Data()) > 0 {
				_ = node.ResetAutoRelayedPeers(db.WithContext(context.TODO()))
			}

			if err := mq.NodeUpdate(modelsNode); err != nil {
				slog.Error("error publishing node update to node", "node", node.ID, "error", err)
			}

			_ = mq.PublishPeerUpdate(false)
		}()

		logic.ReturnSuccessResponseWithJson(w, r, modelsNode.ConvertToAPINode(), "auto assigned gateway")
		return
	}

	if node.RelayedByNodeID != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("node is already using a gw"), "badrequest"))
		return
	}

	gateway := &schema.Node{
		ID: gatewayID,
	}
	err = gateway.Get(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	if gateway.NetworkID != node.NetworkID {
		err = fmt.Errorf("gateway doesn't belong to the node network")
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	if !gateway.IsGateway {
		err = fmt.Errorf("node %s is not a gateway", nodeID)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	node.RelayedByNodeID = &gatewayID
	err = node.AssignGateway(r.Context())
	if err != nil {
		err = fmt.Errorf("failed to assign gateway (%s) to node (%s): %v", gatewayID, node.ID, err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	logger.Log(1, r.Header.Get("user"),
		fmt.Sprintf("assigned nodes to gateway [%s] on network [%s]",
			nodeID, networkName))

	logic.LogEvent(&models.Event{
		Action: schema.GatewayAssign,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   node.ID,
			Name: node.Host.Name,
			Type: schema.GatewaySub,
		},
		Origin: schema.Dashboard,
	})

	modelsNodes := logic.ConvertSchemaNodeToModelsNode(node)

	go func() {
		if len(node.AutoRelayedPeers.Data()) > 0 {
			_ = node.ResetAutoRelayedPeers(db.WithContext(context.TODO()))
		}
		if err := mq.NodeUpdate(modelsNodes); err != nil {
			slog.Error("error publishing node update to node", "node", node.ID, "error", err)
		}
		mq.PublishPeerUpdate(false)
	}()

	logic.ReturnSuccessResponseWithJson(w, r, modelsNodes.ConvertToAPINode(), "assigned gateway")
}

// @Summary     Unassign client nodes from a gateway
// @Router      /api/nodes/{network}/{nodeid}/gateway/unassign [post]
// @Tags        Gateways
// @Security    oauth
// @Produce     json
// @Param       network path string true "Network ID"
// @Param       nodeid path string true "Node ID"
// @Success     200 {object} models.ApiNode
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func unassignGw(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	nodeID := params["nodeid"]
	networkName := params["network"]

	node := &schema.Node{
		ID: nodeID,
	}
	err := node.Get(r.Context(), dbtypes.WithAllPreloads())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	if node.Network.Name != networkName {
		err = fmt.Errorf("network url param does not match node network")
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	if node.AutoAssignGateway {
		node.AutoAssignGateway = false
		err = node.ResetAutoAssignGateway(r.Context())
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
			return
		}

		if node.RelayedByNodeID == nil {
			modelsNode := logic.ConvertSchemaNodeToModelsNode(node)

			go func() {
				if err := mq.NodeUpdate(modelsNode); err != nil {
					slog.Error("error publishing node update to node", "node", node.ID, "error", err)
				}
				_ = mq.PublishPeerUpdate(false)
			}()

			logic.ReturnSuccessResponseWithJson(w, r, modelsNode.ConvertToAPINode(), "unassigned gateway")
			return
		}
	}

	node.RelayedByNodeID = nil
	node.IsGateway = false
	err = node.UnassignGateway(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	logger.Log(1, r.Header.Get("user"),
		fmt.Sprintf("unassigned client nodes from gateway [%s] on network [%s]",
			nodeID, networkName))

	logic.LogEvent(&models.Event{
		Action: schema.GatewayUnAssign,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   node.ID,
			Name: node.Host.Name,
			Type: schema.GatewaySub,
		},
		Origin: schema.Dashboard,
	})

	modelsNode := logic.ConvertSchemaNodeToModelsNode(node)

	go func() {
		if err := mq.NodeUpdate(modelsNode); err != nil {
			slog.Error("error publishing node update to node", "node", node.ID, "error", err)
		}
		_ = mq.PublishPeerUpdate(false)
	}()

	logic.ReturnSuccessResponseWithJson(w, r, modelsNode.ConvertToAPINode(), "unassigned gateway")
}
