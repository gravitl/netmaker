package controller

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
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
	relay.NodeID = params["nodeid"]
	node, err := logic.CreateRelay(relay)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "created relay on node", relay.NodeID, "on network", relay.NetID)
	relayedNodes, err := logic.GetNodesByAddress(relay.NetID, relay.RelayAddrs)
	for _, node := range relayedNodes {
		err = mq.NodeUpdate(&node)
		if err != nil {
			logger.Log(1, "error sending update to relayed node ", node.Address, "on network", relay.NetID, ": ", err.Error())
		}
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
	runUpdates(&node, true)
}

func deleteRelay(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	nodeid := params["nodeid"]
	netid := params["network"]
	node, err := logic.DeleteRelay(netid, nodeid)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "deleted relay server", nodeid, "on network", netid)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
	runUpdates(&node, true)
}
