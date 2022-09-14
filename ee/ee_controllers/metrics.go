package ee_controllers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

// MetricHandlers - How we handle EE Metrics
func MetricHandlers(r *mux.Router) {
	r.HandleFunc("/api/metrics/{network}/{nodeid}", logic.SecurityCheck(true, http.HandlerFunc(getNodeMetrics))).Methods("GET")
	r.HandleFunc("/api/metrics/{network}", logic.SecurityCheck(true, http.HandlerFunc(getNetworkNodesMetrics))).Methods("GET")
	r.HandleFunc("/api/metrics", logic.SecurityCheck(true, http.HandlerFunc(getAllMetrics))).Methods("GET")
}

// get the metrics of a given node
func getNodeMetrics(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	nodeID := params["nodeid"]

	logger.Log(1, r.Header.Get("user"), "requested fetching metrics for node", nodeID, "on network", params["network"])
	metrics, err := logic.GetMetrics(nodeID)
	if err != nil {
		logger.Log(1, r.Header.Get("user"), "failed to fetch metrics of node", nodeID, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logger.Log(1, r.Header.Get("user"), "fetched metrics for node", params["nodeid"])
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(metrics)
}

// get the metrics of all nodes in given network
func getNetworkNodesMetrics(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	network := params["network"]

	logger.Log(1, r.Header.Get("user"), "requested fetching network node metrics on network", network)
	networkNodes, err := logic.GetNetworkNodes(network)
	if err != nil {
		logger.Log(1, r.Header.Get("user"), "failed to fetch metrics of all nodes in network", network, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	networkMetrics := models.NetworkMetrics{}
	networkMetrics.Nodes = make(models.MetricsMap)

	for i := range networkNodes {
		id := networkNodes[i].ID
		metrics, err := logic.GetMetrics(id)
		if err != nil {
			logger.Log(1, r.Header.Get("user"), "failed to append metrics of node", id, "during network metrics fetch", err.Error())
			continue
		}
		networkMetrics.Nodes[id] = *metrics
	}

	logger.Log(1, r.Header.Get("user"), "fetched metrics for network", network)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(networkMetrics)
}

// get Metrics of all nodes on server, lots of data
func getAllMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	logger.Log(1, r.Header.Get("user"), "requested fetching all metrics")

	allNodes, err := logic.GetAllNodes()
	if err != nil {
		logger.Log(1, r.Header.Get("user"), "failed to fetch metrics of all nodes on server", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	networkMetrics := models.NetworkMetrics{}
	networkMetrics.Nodes = make(models.MetricsMap)

	for i := range allNodes {
		id := allNodes[i].ID
		metrics, err := logic.GetMetrics(id)
		if err != nil {
			logger.Log(1, r.Header.Get("user"), "failed to append metrics of node", id, "during all nodes metrics fetch", err.Error())
			continue
		}
		networkMetrics.Nodes[id] = *metrics
	}

	logger.Log(1, r.Header.Get("user"), "fetched metrics for all nodes on server")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(networkMetrics)
}
