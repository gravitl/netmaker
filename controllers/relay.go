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
	go func() {
		// update relay node
		host := logic.GetHostByNodeID(node.ID.String())
		if err := mq.NodeUpdate(&node); err != nil {
			logger.Log(1, "relay node update", host.Name, "on network", relay.NetID, ": ", err.Error())
		}
		// update relayed nodes
		for _, relayedNode := range updatenodes {
			err = mq.NodeUpdate(&relayedNode)
			if err != nil {
				logger.Log(1, "relayed node update ", relayedNode.ID.String(), "on network", relay.NetID, ": ", err.Error())
			}
		}
		// peer updates
		relay := models.Client{
			Host: *host,
			Node: node,
		}
		clients := logic.GetNetworkClients(relay.Node.Network)
		if err := mq.PublishRelayPeerUpdate(&relay, &clients); err != nil {
			logger.Log(1, "peer update to relayed node ", host.Name, "on network", relay.Node.Network, ": ", err.Error())
		}
	}()

	logger.Log(1, r.Header.Get("user"), "created relay on node", relay.NodeID, "on network", relay.NetID)
	apiNode := node.ConvertToAPINode()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
	//runUpdates(&node, true)
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
	go func() {
		//update relay node
		host := logic.GetHostByNodeID(node.ID.String())
		if err := mq.NodeUpdate(&node); err != nil {
			logger.Log(1, "relay node update", host.Name, "on network", node.Network, ": ", err.Error())
		}
		//update relayed nodes
		for _, relayedNode := range updatenodes {
			err = mq.NodeUpdate(&relayedNode)
			if err != nil {
				logger.Log(1, "relayed node update ", relayedNode.ID.String(), "on network", relayedNode.Network, ": ", err.Error())
			}
		}
		// peer updates
		relay := models.Client{
			Host: *host,
			Node: node,
		}
		clients := logic.GetNetworkClients(relay.Node.Network)
		if err := mq.PublishRelayPeerUpdate(&relay, &clients); err != nil {
			logger.Log(1, "peer update to relayed node ", host.Name, "on network", relay.Node.Network, ": ", err.Error())
		}
	}()

	logger.Log(1, r.Header.Get("user"), "deleted relay on node", node.ID.String(), "on network", node.Network)
	apiNode := node.ConvertToAPINode()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
	//runUpdates(&node, true)
}
