package controllers

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"net/http"
)

func NetworkHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/networks/{network}/graph", logic.SecurityCheck(true, http.HandlerFunc(getNetworkGraph))).Methods(http.MethodGet)
}

func getNetworkGraph(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	network := params["network"]
	networkNodes, err := logic.GetNetworkNodes(network)
	if err != nil {
		logger.Log(1, r.Header.Get("user"), "failed to get network nodes", err.Error())
		return
	}
	networkNodes = logic.AddStaticNodestoList(networkNodes)
	// return all the nodes in JSON/API format
	apiNodes := logic.GetAllNodesAPIWithLocation(networkNodes[:])
	logic.SortApiNodes(apiNodes[:])
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNodes)
}
