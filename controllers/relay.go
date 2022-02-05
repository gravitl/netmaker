package controller

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

func createRelay(w http.ResponseWriter, r *http.Request) {
	var relay models.RelayRequest
	var params = mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	err := json.NewDecoder(r.Body).Decode(&relay)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	relay.NetID = params["network"]
	relay.NodeID = params["macaddress"]
	node, err := logic.CreateRelay(relay)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "created relay on node", relay.NodeID, "on network", relay.NetID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

func deleteRelay(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	nodeMac := params["macaddress"]
	netid := params["network"]
	node, err := logic.DeleteRelay(netid, nodeMac)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "deleted egress gateway", nodeMac, "on network", netid)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}
