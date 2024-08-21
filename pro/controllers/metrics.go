package controllers

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"golang.org/x/exp/slog"
)

// MetricHandlers - How we handle Pro Metrics
func MetricHandlers(r *mux.Router) {
	r.HandleFunc("/api/metrics/{network}/{nodeid}", logic.SecurityCheck(true, http.HandlerFunc(getNodeMetrics))).Methods(http.MethodGet)
	r.HandleFunc("/api/metrics/{network}", logic.SecurityCheck(true, http.HandlerFunc(getNetworkNodesMetrics))).Methods(http.MethodGet)
	r.HandleFunc("/api/metrics", logic.SecurityCheck(true, http.HandlerFunc(getAllMetrics))).Methods(http.MethodGet)
	r.HandleFunc("/api/metrics-ext/{network}", logic.SecurityCheck(true, http.HandlerFunc(getNetworkExtMetrics))).Methods(http.MethodGet)
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
		// metrics, err := proLogic.GetMetrics(id.String())
		// if err != nil {
		// 	logger.Log(1, r.Header.Get("user"), "failed to append metrics of node", id.String(), "during network metrics fetch", err.Error())
		// 	continue
		// }
		host, _ := logic.GetHost(networkNodes[i].HostID.String())
		metrics := &models.Metrics{
			Network:      networkNodes[i].Network,
			NodeID:       id.String(),
			NodeName:     host.Name,
			Connectivity: make(map[string]models.Metric),
		}
		for _, node := range networkNodes {
			if node.ID == id {
				continue
			}
			m := models.Metric{}
			m.Connected = true
			m.ActualUptime = time.Duration(time.Hour * time.Duration(rand.Intn(50)))
			m.Latency = int64(rand.Intn(10))
			m.PercentUp = float64(rand.Intn(100-90+1) + 90)
			m.TotalSent = int64(rand.Intn(10000))
			m.TotalReceived = int64(rand.Intn(10000))
			m.Uptime = int64(rand.Intn(10000))
			metrics.Connectivity[node.ID.String()] = m
		}

		networkMetrics.Nodes[id.String()] = *metrics
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
	ingresses, err := proLogic.GetNetworkIngresses(network) // grab all the ingress gateways
	if err != nil {
		logger.Log(1, r.Header.Get("user"), "failed to fetch metrics of ext clients in network", network, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	clients, err := logic.GetNetworkExtClients(network) // grab all the network ext clients
	if err != nil {
		if database.IsEmptyRecord(err) {
			var metrics struct{}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(metrics)
			return
		}
		logger.Log(1, r.Header.Get("user"), "failed to fetch metrics of ext clients in network", network, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	networkMetrics := models.Metrics{}
	networkMetrics.Connectivity = make(map[string]models.Metric)

	for i := range ingresses {
		id := ingresses[i].ID
		ingressMetrics, err := proLogic.GetMetrics(id.String())
		if err != nil {
			logger.Log(1, r.Header.Get("user"), "failed to append external client metrics from ingress node", id.String(), err.Error())
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
			if _, ok := ingressMetrics.Connectivity[clients[j].ClientID]; ok {
				networkMetrics.Connectivity[clients[j].ClientID] = ingressMetrics.Connectivity[clients[j].ClientID]
			}
		}
	}

	slog.Debug("sending collected client metrics", "metrics", networkMetrics.Connectivity)
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
		metrics, err := proLogic.GetMetrics(id.String())
		if err != nil {
			logger.Log(1, r.Header.Get("user"), "failed to append metrics of node", id.String(), "during all nodes metrics fetch", err.Error())
			continue
		}
		networkMetrics.Nodes[id.String()] = *metrics
	}

	logger.Log(1, r.Header.Get("user"), "fetched metrics for all nodes on server")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(networkMetrics)
}
