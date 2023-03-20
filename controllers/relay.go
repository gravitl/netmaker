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
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
func createRelay(w http.ResponseWriter, r *http.Request) {
	var relay models.RelayRequest
	var params = mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	err := json.NewDecoder(r.Body).Decode(&relay)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	relay.NetID = params["network"]
	relay.NodeID = params["nodeid"]
	updatenodes, node, err := logic.CreateRelay(relay)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to create relay on node [%s] on network [%s]: %v", relay.NodeID, relay.NetID, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logger.Log(1, r.Header.Get("user"), "created relay on node", relay.NodeID, "on network", relay.NetID)
	for _, relayedNode := range updatenodes {
		err = mq.NodeUpdate(&relayedNode)
		if err != nil {
			logger.Log(1, "error sending update to relayed node ", relayedNode.ID.String(), "on network", relay.NetID, ": ", err.Error())
		}
	}

	apiNode := node.ConvertToAPINode()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
	runUpdates(&node, true, false)
}

// swagger:route DELETE /api/nodes/{network}/{nodeid}/deleterelay nodes deleteRelay
//
// Remove a relay.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
func deleteRelay(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	nodeid := params["nodeid"]
	netid := params["network"]
	updatenodes, node, err := logic.DeleteRelay(netid, nodeid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "deleted relay server", nodeid, "on network", netid)
	for _, relayedNode := range updatenodes {
		err = mq.NodeUpdate(&relayedNode)
		if err != nil {
			logger.Log(1, "error sending update to relayed node ", relayedNode.ID.String(), "on network", netid, ": ", err.Error())
		}
	}
	apiNode := node.ConvertToAPINode()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
	runUpdates(&node, true, false)
}

// swagger:route POST /api/hosts/{hostid}/relay hosts createHostRelay
//
// Create a relay.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
func createHostRelay(w http.ResponseWriter, r *http.Request) {
	var relay models.HostRelayRequest
	var params = mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	err := json.NewDecoder(r.Body).Decode(&relay)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	relay.HostID = params["hostid"]
	relayHost, _, err := logic.CreateHostRelay(relay)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to create relay on host [%s]: %v", relay.HostID, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	if err := mq.HostUpdate(&models.HostUpdate{
		Action: models.UpdateHost,
		Host:   *relayHost,
	}); err != nil {
		logger.Log(0, "failed to send host update: ", relayHost.ID.String(), err.Error())
	}
	logger.Log(1, r.Header.Get("user"), "created relay on host", relay.HostID)
	go func(relayHost *models.Host) {
		relatedhosts := logic.GetRelatedHosts(relayHost.ID.String())
		for _, relatedHost := range relatedhosts {
			relatedHost := relatedHost
			relatedHost.ProxyEnabled = true
			logic.UpsertHost(&relatedHost)
			if err := mq.HostUpdate(&models.HostUpdate{
				Action: models.UpdateHost,
				Host:   relatedHost,
			}); err != nil {
				logger.Log(0, "failed to send host update: ", relatedHost.ID.String(), err.Error())
			}
		}
		if err := mq.PublishPeerUpdateForHost("", relayHost, nil, nil); err != nil {
			logger.Log(0, "fail to publish peer update: ", err.Error())
		}
	}(relayHost)

	apiHostData := relayHost.ConvertNMHostToAPI()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiHostData)
}

// swagger:route DELETE /api/hosts/{hostid}/relay hosts deleteHostRelay
//
// Remove a relay.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
func deleteHostRelay(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	hostid := params["hostid"]
	relayHost, _, err := logic.DeleteHostRelay(hostid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "deleted relay host", hostid)
	go func() {
		if err := mq.PublishPeerUpdateForHost("", relayHost, nil, nil); err != nil {
			logger.Log(0, "failed to update peers after relay delete:", err.Error())
		}
	}()
	apiHostData := relayHost.ConvertNMHostToAPI()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiHostData)
}
