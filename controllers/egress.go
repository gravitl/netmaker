package controller

import (
	"encoding/json"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logic"
)

type RoutingNode struct {
	NodeID      string `json:"node_id"`
	RouteMetric uint32 `json:"route_metric"`
}

type RoutingGrp struct {
	GrpID       string `json:"group_id"`
	RouteMetric uint32 `json:"route_metric"`
}

type Egress struct {
	Network      net.IPNet
	RoutingNodes []RoutingNode
	RoutingGrp   []RoutingGrp
}

func egressHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/egress/{network}", logic.SecurityCheck(true, http.HandlerFunc(createEgressNetwork))).Methods(http.MethodPost)
}

func createEgressNetwork(w http.ResponseWriter, r *http.Request) {
	var req Egress
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if len(req.RoutingNodes) > 0 {
		for _, routingNodeI := range req.RoutingNodes {
			node, err := logic.GetNodeByID(routingNodeI.NodeID)
			if err != nil {
				continue
			}
			node.IsGw = true
			if !logic.StringSliceContains(node.EgressGatewayRanges, req.Network.String()) {
				node.EgressGatewayRanges = append(node.EgressGatewayRanges, req.Network.String())
			}
			logic.UpsertNode(&node)
		}
	}
}
