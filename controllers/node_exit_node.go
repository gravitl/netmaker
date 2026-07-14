package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/middleware"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/scope"
)

func nodeExitNodeHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/nodes/{network}/{nodeid}/exit_nodes",
		middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(listNodeExitNodes)))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/nodes/{network}/{nodeid}/exit_node",
		middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(getNodeExitNode)))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/nodes/{network}/{nodeid}/exit_node",
		middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(assignNodeExitNode)))).
		Methods(http.MethodPut)
}

// @Summary     List internet exit nodes available for a node
// @Router      /api/v1/nodes/{network}/{nodeid}/exit_nodes [get]
// @Tags        Nodes
// @Security    oauth
// @Produce     json
// @Param       network path string true "Network ID"
// @Param       nodeid  path string true "Node ID"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
func listNodeExitNodes(w http.ResponseWriter, r *http.Request) {
	network, nodeID, ok := nodeExitNodeParams(w, r)
	if !ok {
		return
	}
	nodes, err := logic.ListNodeExitNodes(db.WithContext(r.Context()), network, nodeID)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, exitNodeErrType(err)))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, nodes, "fetched exit nodes")
}

// @Summary     Get the internet exit node assigned to a node
// @Router      /api/v1/nodes/{network}/{nodeid}/exit_node [get]
// @Tags        Nodes
// @Security    oauth
// @Produce     json
// @Param       network path string true "Network ID"
// @Param       nodeid  path string true "Node ID"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
func getNodeExitNode(w http.ResponseWriter, r *http.Request) {
	network, nodeID, ok := nodeExitNodeParams(w, r)
	if !ok {
		return
	}
	selected, err := logic.GetNodeExitNode(db.WithContext(r.Context()), network, nodeID)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, exitNodeErrType(err)))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, selected, "fetched selected exit node")
}

// @Summary     Assign or clear the internet exit node for a node
// @Router      /api/v1/nodes/{network}/{nodeid}/exit_node [put]
// @Tags        Nodes
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       network path string true "Network ID"
// @Param       nodeid  path string true "Node ID"
// @Param       body body models.NodeExitNodeSelectionReq true "Exit node assignment"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
func assignNodeExitNode(w http.ResponseWriter, r *http.Request) {
	network, nodeID, ok := nodeExitNodeParams(w, r)
	if !ok {
		return
	}
	var req models.NodeExitNodeSelectionReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}
	selected, err := logic.AssignNodeExitNode(db.WithContext(r.Context()), network, nodeID, req.EgressID)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, exitNodeErrType(err)))
		return
	}
	msg := "exit node selected"
	if req.EgressID == "" {
		msg = "exit node cleared"
	}
	logic.ReturnSuccessResponseWithJson(w, r, selected, msg)
}

func nodeExitNodeParams(w http.ResponseWriter, r *http.Request) (network, nodeID string, ok bool) {
	vars := mux.Vars(r)
	network = vars["network"]
	nodeID = vars["nodeid"]
	if network == "" || nodeID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network and node are required"), logic.BadReq))
		return "", "", false
	}
	return network, nodeID, true
}

func exitNodeErrType(err error) logic.ApiErrorType {
	switch err.Error() {
	case "network and node are required",
		"node not found",
		"node not in network",
		"exit node not found",
		"egress is not an active internet exit node in this network",
		"routing node cannot select itself as exit node",
		"internet egress has no routing node",
		"gateway nodes cannot be assigned an exit node",
		"node is relayed by a different gateway":
		return logic.BadReq
	default:
		return logic.Internal
	}
}
