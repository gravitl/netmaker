package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
)

// swagger:route POST /api/nodes/{network}/{nodeid}/createrelay nodes createRelay
//
// Create a relay.
//
//		Schemes: https
//
// 		Security:
//   		oauth
func createRelay(w http.ResponseWriter, r *http.Request) {
	var relay models.RelayRequest
	var params = mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	err := json.NewDecoder(r.Body).Decode(&relay)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	relay.NetID = params["network"]
	relay.NodeID = params["nodeid"]
	updatenodes, node, err := logic.CreateRelay(relay)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to create relay on node [%s] on network [%s]: %v", relay.NodeID, relay.NetID, err))
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "created relay on node", relay.NodeID, "on network", relay.NetID)
	for _, relayedNode := range updatenodes {
		err = mq.NodeUpdate(&relayedNode)
		if err != nil {
			logger.Log(1, "error sending update to relayed node ", relayedNode.Name, "on network", relay.NetID, ": ", err.Error())
		}
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
	runUpdates(&node, true)
}

// swagger:route DELETE /api/nodes/{network}/{nodeid}/deleterelay nodes deleteRelay
//
// Remove a relay.
//
//		Schemes: https
//
// 		Security:
//   		oauth
func deleteRelay(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	nodeid := params["nodeid"]
	netid := params["network"]
	updatenodes, node, err := logic.DeleteRelay(netid, nodeid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "deleted relay server", nodeid, "on network", netid)
	for _, relayedNode := range updatenodes {
		err = mq.NodeUpdate(&relayedNode)
		if err != nil {
			logger.Log(1, "error sending update to relayed node ", relayedNode.Name, "on network", netid, ": ", err.Error())
		}
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
	runUpdates(&node, true)
}
