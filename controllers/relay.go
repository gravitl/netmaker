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
	if err = runServerPeerUpdate(relay.NetID, isServer(&node), "relay create"); err != nil {
		logger.Log(1, "internal error when creating relay on node:", relay.NodeID)
	}
	go func() {
		if err := mq.NodeUpdate(&node); err != nil {
			logger.Log(1, "error publishing node update", err.Error())
		}
		if err := mq.PublishPeerUpdate(&node); err != nil {
			logger.Log(1, "error publishing peer update ", err.Error())
		}
	}()
	logger.Log(1, r.Header.Get("user"), "created relay on node", relay.NodeID, "on network", relay.NetID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
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
	if err = runServerPeerUpdate(netid, isServer(&node), "relay delete"); err != nil {
		logger.Log(1, "internal error when deleting relay on node:", nodeid)
	}
	go func() {
		if err := mq.NodeUpdate(&node); err != nil {
			logger.Log(1, "error publishing node update", err.Error())
		}
		if err := mq.PublishPeerUpdate(&node); err != nil {
			logger.Log(1, "error publishing peer update ", err.Error())
		}
	}()
	logger.Log(1, r.Header.Get("user"), "deleted relay server", nodeid, "on network", netid)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}
