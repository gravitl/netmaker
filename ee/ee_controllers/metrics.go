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
	r.HandleFunc("/api/metrics-ext/{network}", logic.SecurityCheck(true, http.HandlerFunc(getNetworkExtMetrics))).Methods("GET")
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

// get the metrics for ext clients on a given network
func getNetworkExtMetrics(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	network := params["network"]

	logger.Log(1, r.Header.Get("user"), "requested fetching external client metrics on network", network)
	ingresses, err := logic.GetNetworkIngresses(network) // grab all the ingress gateways
	if err != nil {
		logger.Log(1, r.Header.Get("user"), "failed to fetch metrics of ext clients in network", network, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	clients, err := logic.GetNetworkExtClients(network) // grab all the network ext clients
	if err != nil {
		logger.Log(1, r.Header.Get("user"), "failed to fetch metrics of ext clients in network", network, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	networkMetrics := models.Metrics{}
	networkMetrics.Connectivity = make(map[string]models.Metric)

	for i := range ingresses {
		id := ingresses[i].ID
		ingressMetrics, err := logic.GetMetrics(id)
		if err != nil {
			logger.Log(1, r.Header.Get("user"), "failed to append external client metrics from ingress node", id, err.Error())
			continue
		}
		if ingressMetrics.Connectivity == nil {
			continue
		}
		for j := range clients {
			if clients[j].Network != network {
				continue
			}
			// if metrics for that client have been reported, append them
			if len(ingressMetrics.Connectivity[clients[j].ClientID].NodeName) > 0 {
				networkMetrics.Connectivity[clients[j].ClientID] = ingressMetrics.Connectivity[clients[j].ClientID]
			}
		}
	}

	logger.Log(1, r.Header.Get("user"), "fetched ext client metrics for network", network)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(networkMetrics.Connectivity)
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
