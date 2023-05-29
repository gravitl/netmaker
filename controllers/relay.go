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
	var relayRequest models.RelayRequest
	var params = mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	err := json.NewDecoder(r.Body).Decode(&relayRequest)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	relayRequest.NetID = params["network"]
	relayRequest.NodeID = params["nodeid"]
	_, relayNode, err := logic.CreateRelay(relayRequest)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to create relay on node [%s] on network [%s]: %v", relayRequest.NodeID, relayRequest.NetID, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	// relay := models.Client{
	// 	Host: *logic.GetHostByNodeID(params["nodeid"]),
	// 	Node: relayNode,
	// }
	// peers, err := logic.GetNetworkClients(relay.Node.Network)
	// if err != nil {
	// 	logger.Log(0, "error getting network nodes: ", err.Error())
	// 	logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
	// 	return
	// }
	//mq.PubPeersforRelay(relay, peers)
	//for _, relayed := range relayedClients {
	//mq.PubPeersForRelayedNode(relayed, relay, peers)
	//}
	// clients := peers
	// for _, client := range clients {
	// 	mq.PubPeerUpdate(&client, &relay, &peers)
	// }
	go mq.PublishPeerUpdate()
	logger.Log(1, r.Header.Get("user"), "created relay on node", relayRequest.NodeID, "on network", relayRequest.NetID)
	apiNode := relayNode.ConvertToAPINode()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
	runUpdates(&relayNode, true)
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
	updateClients, node, err := logic.DeleteRelay(netid, nodeid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "deleted relay server", nodeid, "on network", netid)
	go func() {
		//update relayHost node
		relayHost := logic.GetHostByNodeID(node.ID.String())
		if err := mq.NodeUpdate(&node); err != nil {
			logger.Log(1, "relay node update", relayHost.Name, "on network", node.Network, ": ", err.Error())
		}
		for _, relayedClient := range updateClients {
			err = mq.NodeUpdate(&relayedClient.Node)
			if err != nil {
				logger.Log(1, "relayed node update ", relayedClient.Node.ID.String(), "on network", relayedClient.Node.Network, ": ", err.Error())

			}
		}
		// peers, err := logic.GetNetworkClients(node.Network)
		// if err != nil {
		// 	logger.Log(0, "error getting network nodes: ", err.Error())
		// 	logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		// 	return
		// }
		// clients := peers
		// for _, client := range clients {
		// 	mq.PubPeerUpdate(&client, nil, &peers)
		// }
		go mq.PublishPeerUpdate()
	}()
	logger.Log(1, r.Header.Get("user"), "deleted relay on node", node.ID.String(), "on network", node.Network)
	apiNode := node.ConvertToAPINode()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
	runUpdates(&node, true)
}
