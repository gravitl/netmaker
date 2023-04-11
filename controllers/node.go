package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/pro"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/models/promodels"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/crypto/bcrypt"
)

var hostIDHeader = "host-id"

func nodeHandlers(r *mux.Router) {

	r.HandleFunc("/api/nodes", authorize(false, false, "user", http.HandlerFunc(getAllNodes))).Methods(http.MethodGet)
	r.HandleFunc("/api/nodes/{network}", authorize(false, true, "network", http.HandlerFunc(getNetworkNodes))).Methods(http.MethodGet)
	r.HandleFunc("/api/nodes/{network}/{nodeid}", authorize(true, true, "node", http.HandlerFunc(getNode))).Methods(http.MethodGet)
	r.HandleFunc("/api/nodes/{network}/{nodeid}", authorize(false, true, "node", http.HandlerFunc(updateNode))).Methods(http.MethodPut)
	r.HandleFunc("/api/nodes/{network}/{nodeid}", authorize(true, true, "node", http.HandlerFunc(deleteNode))).Methods(http.MethodDelete)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/createrelay", authorize(false, true, "user", http.HandlerFunc(createRelay))).Methods(http.MethodPost)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/deleterelay", authorize(false, true, "user", http.HandlerFunc(deleteRelay))).Methods(http.MethodDelete)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/creategateway", authorize(false, true, "user", http.HandlerFunc(createEgressGateway))).Methods(http.MethodPost)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/deletegateway", authorize(false, true, "user", http.HandlerFunc(deleteEgressGateway))).Methods(http.MethodDelete)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/createingress", logic.SecurityCheck(false, http.HandlerFunc(createIngressGateway))).Methods(http.MethodPost)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/deleteingress", logic.SecurityCheck(false, http.HandlerFunc(deleteIngressGateway))).Methods(http.MethodDelete)
	r.HandleFunc("/api/nodes/{network}/{nodeid}", authorize(true, true, "node", http.HandlerFunc(updateNode))).Methods(http.MethodPost)
	r.HandleFunc("/api/nodes/adm/{network}/authenticate", authenticate).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/nodes/migrate", migrate).Methods(http.MethodPost)
}

// swagger:route POST /api/nodes/adm/{network}/authenticate nodes authenticate
//
// Authenticate to make further API calls related to a network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: successResponse
func authenticate(response http.ResponseWriter, request *http.Request) {

	var authRequest models.AuthParams
	var result models.Node
	var errorResponse = models.ErrorResponse{
		Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
	}

	decoder := json.NewDecoder(request.Body)
	decoderErr := decoder.Decode(&authRequest)
	defer request.Body.Close()

	if decoderErr != nil {
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = decoderErr.Error()
		logger.Log(0, request.Header.Get("user"), "error decoding request body: ",
			decoderErr.Error())
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}
	errorResponse.Code = http.StatusBadRequest
	if authRequest.ID == "" {
		errorResponse.Message = "W1R3: ID can't be empty"
		logger.Log(0, request.Header.Get("user"), errorResponse.Message)
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	} else if authRequest.Password == "" {
		errorResponse.Message = "W1R3: Password can't be empty"
		logger.Log(0, request.Header.Get("user"), errorResponse.Message)
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}
	var err error
	result, err = logic.GetNodeByID(authRequest.ID)
	if err != nil {
		result, err = logic.GetDeletedNodeByID(authRequest.ID)
		if err != nil {
			errorResponse.Code = http.StatusBadRequest
			errorResponse.Message = err.Error()
			logger.Log(0, request.Header.Get("user"),
				fmt.Sprintf("failed to get node info [%s]: %v", authRequest.ID, err))
			logic.ReturnErrorResponse(response, request, errorResponse)
			return
		}
	}
	host, err := logic.GetHost(result.HostID.String())
	if err != nil {
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logger.Log(0, request.Header.Get("user"),
			"error retrieving host: ", err.Error())
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}
	err = bcrypt.CompareHashAndPassword([]byte(host.HostPass), []byte(authRequest.Password))
	if err != nil {
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logger.Log(0, request.Header.Get("user"),
			"error validating user password: ", err.Error())
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}

	tokenString, err := logic.CreateJWT(authRequest.ID, authRequest.MacAddress, result.Network)
	if tokenString == "" {
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = "Could not create Token"
		logger.Log(0, request.Header.Get("user"),
			fmt.Sprintf("%s: %v", errorResponse.Message, err))
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}

	var successResponse = models.SuccessResponse{
		Code:    http.StatusOK,
		Message: "W1R3: Device " + authRequest.ID + " Authorized",
		Response: models.SuccessfulLoginResponse{
			AuthToken: tokenString,
			ID:        authRequest.ID,
		},
	}
	successJSONResponse, jsonError := json.Marshal(successResponse)

	if jsonError != nil {
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logger.Log(0, request.Header.Get("user"),
			"error marshalling resp: ", err.Error())
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}
	response.WriteHeader(http.StatusOK)
	response.Header().Set("Content-Type", "application/json")
	response.Write(successJSONResponse)
}

// The middleware for most requests to the API
// They all pass  through here first
// This will validate the JWT (or check for master token)
// This will also check against the authNetwork and make sure the node should be accessing that endpoint,
// even if it's technically ok
// This is kind of a poor man's RBAC. There's probably a better/smarter way.
// TODO: Consider better RBAC implementations
func authorize(hostAllowed, networkCheck bool, authNetwork string, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusUnauthorized, Message: logic.Unauthorized_Msg,
		}

		var params = mux.Vars(r)

		networkexists, _ := logic.NetworkExists(params["network"])
		//check that the request is for a valid network
		//if (networkCheck && !networkexists) || err != nil {
		if networkCheck && !networkexists {
			logic.ReturnErrorResponse(w, r, errorResponse)
			return
		} else {
			w.Header().Set("Content-Type", "application/json")

			//get the auth token
			bearerToken := r.Header.Get("Authorization")

			var tokenSplit = strings.Split(bearerToken, " ")

			//I put this in in case the user doesn't put in a token at all (in which case it's empty)
			//There's probably a smarter way of handling this.
			var authToken = "928rt238tghgwe@TY@$Y@#WQAEGB2FC#@HG#@$Hddd"

			if len(tokenSplit) > 1 {
				authToken = tokenSplit[1]
			} else {
				logic.ReturnErrorResponse(w, r, errorResponse)
				return
			}
			// check if host instead of user
			if hostAllowed {
				// TODO --- should ensure that node is only operating on itself
				if hostID, _, _, err := logic.VerifyHostToken(authToken); err == nil {
					r.Header.Set(hostIDHeader, hostID)
					// this indicates request is from a node
					// used for failover - if a getNode comes from node, this will trigger a metrics wipe
					next.ServeHTTP(w, r)
					return
				}
			}

			var isAuthorized = false
			var nodeID = ""
			username, networks, isadmin, errN := logic.VerifyUserToken(authToken)
			if errN != nil {
				logic.ReturnErrorResponse(w, r, errorResponse)
				return
			}

			isnetadmin := isadmin
			if errN == nil && isadmin {
				nodeID = "mastermac"
				isAuthorized = true
				r.Header.Set("ismasterkey", "yes")
			}
			if !isadmin && params["network"] != "" {
				if logic.StringSliceContains(networks, params["network"]) && pro.IsUserNetAdmin(params["network"], username) {
					isnetadmin = true
				}
			}
			//The mastermac (login with masterkey from config) can do everything!! May be dangerous.
			if nodeID == "mastermac" {
				isAuthorized = true
				r.Header.Set("ismasterkey", "yes")
				//for everyone else, there's poor man's RBAC. The "cases" are defined in the routes in the handlers
				//So each route defines which access network should be allowed to access it
			} else {
				switch authNetwork {
				case "all":
					isAuthorized = true
				case "nodes":
					isAuthorized = (nodeID != "") || isnetadmin
				case "network":
					if isnetadmin {
						isAuthorized = true
					} else {
						node, err := logic.GetNodeByID(nodeID)
						if err != nil {
							logic.ReturnErrorResponse(w, r, errorResponse)
							return
						}
						isAuthorized = (node.Network == params["network"])
					}
				case "node":
					if isnetadmin {
						isAuthorized = true
					} else {
						isAuthorized = (nodeID == params["netid"])
					}
				case "host":
				case "user":
					isAuthorized = true
				default:
					isAuthorized = false
				}
			}
			if !isAuthorized {
				logic.ReturnErrorResponse(w, r, errorResponse)
				return
			} else {
				//If authorized, this function passes along it's request and output to the appropriate route function.
				if username == "" {
					username = "(user not found)"
				}
				r.Header.Set("user", username)
				next.ServeHTTP(w, r)
			}
		}
	}
}

// swagger:route GET /api/nodes/{network} nodes getNetworkNodes
//
// Gets all nodes associated with network including pending nodes.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeSliceResponse
func getNetworkNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	networkName := params["network"]
	nodes, err := logic.GetNetworkNodes(networkName)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("error fetching nodes on network %s: %v", networkName, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	// returns all the nodes in JSON/API format
	apiNodes := logic.GetAllNodesAPI(nodes[:])
	logger.Log(2, r.Header.Get("user"), "fetched nodes on network", networkName)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNodes)
}

// swagger:route GET /api/nodes nodes getAllNodes
//
// Get all nodes across all networks.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeSliceResponse
//
// Not quite sure if this is necessary. Probably necessary based on front end but may want to review after iteration 1 if it's being used or not
func getAllNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user, err := logic.GetUser(r.Header.Get("user"))
	if err != nil && r.Header.Get("ismasterkey") != "yes" {
		logger.Log(0, r.Header.Get("user"),
			"error fetching user info: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	var nodes []models.Node
	if user.IsAdmin || r.Header.Get("ismasterkey") == "yes" {
		nodes, err = logic.GetAllNodes()
		if err != nil {
			logger.Log(0, "error fetching all nodes info: ", err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	} else {
		nodes, err = getUsersNodes(*user)
		if err != nil {
			logger.Log(0, r.Header.Get("user"),
				"error fetching nodes: ", err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	}
	// return all the nodes in JSON/API format
	apiNodes := logic.GetAllNodesAPI(nodes[:])
	logger.Log(3, r.Header.Get("user"), "fetched all nodes they have access to")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNodes)
}

func getUsersNodes(user models.User) ([]models.Node, error) {
	var nodes []models.Node
	var err error
	for _, networkName := range user.Networks {
		tmpNodes, err := logic.GetNetworkNodes(networkName)
		if err != nil {
			continue
		}
		nodes = append(nodes, tmpNodes...)
	}
	return nodes, err
}

// swagger:route GET /api/nodes/{network}/{nodeid} nodes getNode
//
// Get an individual node.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
func getNode(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")
	nodeRequest := r.Header.Get("requestfrom") == "node"

	var params = mux.Vars(r)
	nodeid := params["nodeid"]
	node, err := logic.GetNodeByID(nodeid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("error fetching node [ %s ] info: %v", nodeid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	host, err := logic.GetHost(node.HostID.String())
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("error fetching host for node [ %s ] info: %v", nodeid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	hostPeerUpdate, err := logic.GetPeerUpdateForHost(context.Background(), node.Network, host, nil, nil)
	if err != nil && !database.IsEmptyRecord(err) {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("error fetching wg peers config for host [ %s ]: %v", host.ID.String(), err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	server := servercfg.GetServerInfo()
	if servercfg.GetBrokerType() == servercfg.EmqxBrokerType {
		// set MQ username for EMQX clients
		server.MQUserName = host.ID.String()
	}
	response := models.NodeGet{
		Node:         node,
		Host:         *host,
		HostPeers:    hostPeerUpdate.Peers,
		Peers:        hostPeerUpdate.NodePeers,
		ServerConfig: server,
		PeerIDs:      hostPeerUpdate.PeerIDs,
	}

	if servercfg.Is_EE && nodeRequest {
		if err = logic.EnterpriseResetAllPeersFailovers(node.ID, node.Network); err != nil {
			logger.Log(1, "failed to reset failover list during node config pull", node.ID.String(), node.Network)
		}
	}

	logger.Log(2, r.Header.Get("user"), "fetched node", params["nodeid"])
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// == EGRESS ==

// swagger:route POST /api/nodes/{network}/{nodeid}/creategateway nodes createEgressGateway
//
// Create an egress gateway.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
func createEgressGateway(w http.ResponseWriter, r *http.Request) {
	var gateway models.EgressGatewayRequest
	var params = mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	err := json.NewDecoder(r.Body).Decode(&gateway)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	gateway.NetID = params["network"]
	gateway.NodeID = params["nodeid"]
	node, err := logic.CreateEgressGateway(gateway)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to create egress gateway on node [%s] on network [%s]: %v",
				gateway.NodeID, gateway.NetID, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	apiNode := node.ConvertToAPINode()
	logger.Log(1, r.Header.Get("user"), "created egress gateway on node", gateway.NodeID, "on network", gateway.NetID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
	go func() {
		mq.PublishPeerUpdate()
	}()
	runUpdates(&node, true)
}

// swagger:route DELETE /api/nodes/{network}/{nodeid}/deletegateway nodes deleteEgressGateway
//
// Delete an egress gateway.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
func deleteEgressGateway(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	nodeid := params["nodeid"]
	netid := params["network"]
	node, err := logic.DeleteEgressGateway(netid, nodeid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to delete egress gateway on node [%s] on network [%s]: %v",
				nodeid, netid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	apiNode := node.ConvertToAPINode()
	logger.Log(1, r.Header.Get("user"), "deleted egress gateway on node", nodeid, "on network", netid)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
	go func() {
		mq.PublishPeerUpdate()
	}()
	runUpdates(&node, true)
}

// == INGRESS ==

// swagger:route POST /api/nodes/{network}/{nodeid}/createingress nodes createIngressGateway
//
// Create an ingress gateway.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
func createIngressGateway(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	nodeid := params["nodeid"]
	netid := params["network"]
	type failoverData struct {
		Failover bool `json:"failover"`
	}
	var failoverReqBody failoverData
	json.NewDecoder(r.Body).Decode(&failoverReqBody)

	node, err := logic.CreateIngressGateway(netid, nodeid, failoverReqBody.Failover)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to create ingress gateway on node [%s] on network [%s]: %v",
				nodeid, netid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	if servercfg.Is_EE && failoverReqBody.Failover {
		if err = logic.EnterpriseResetFailoverFunc(node.Network); err != nil {
			logger.Log(1, "failed to reset failover list during failover create", node.ID.String(), node.Network)
		}
	}

	apiNode := node.ConvertToAPINode()
	logger.Log(1, r.Header.Get("user"), "created ingress gateway on node", nodeid, "on network", netid)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)

	runUpdates(&node, true)
}

// swagger:route DELETE /api/nodes/{network}/{nodeid}/deleteingress nodes deleteIngressGateway
//
// Delete an ingress gateway.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
func deleteIngressGateway(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	nodeid := params["nodeid"]
	netid := params["network"]
	node, wasFailover, removedClients, err := logic.DeleteIngressGateway(netid, nodeid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to delete ingress gateway on node [%s] on network [%s]: %v",
				nodeid, netid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	if servercfg.Is_EE && wasFailover {
		if err = logic.EnterpriseResetFailoverFunc(node.Network); err != nil {
			logger.Log(1, "failed to reset failover list during failover create", node.ID.String(), node.Network)
		}
	}

	apiNode := node.ConvertToAPINode()
	logger.Log(1, r.Header.Get("user"), "deleted ingress gateway", nodeid)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)

	if len(removedClients) > 0 {
		host, err := logic.GetHost(node.HostID.String())
		if err == nil {
			go mq.PublishSingleHostPeerUpdate(
				context.Background(),
				host,
				nil,
				removedClients[:],
			)
		}
	}

	runUpdates(&node, true)
}

// swagger:route PUT /api/nodes/{network}/{nodeid} nodes updateNode
//
// Update an individual node.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
func updateNode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	//start here
	nodeid := params["nodeid"]
	currentNode, err := logic.GetNodeByID(nodeid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("error fetching node [ %s ] info: %v", nodeid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	var newData models.ApiNode
	// we decode our body request params
	err = json.NewDecoder(r.Body).Decode(&newData)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	newNode := newData.ConvertToServerNode(&currentNode)
	relayupdate := false
	if currentNode.IsRelay && len(newNode.RelayAddrs) > 0 {
		if len(newNode.RelayAddrs) != len(currentNode.RelayAddrs) {
			relayupdate = true
		} else {
			for i, addr := range newNode.RelayAddrs {
				if addr != currentNode.RelayAddrs[i] {
					relayupdate = true
				}
			}
		}
	}
	host, err := logic.GetHost(newNode.HostID.String())
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get host for node  [ %s ] info: %v", nodeid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	relayedUpdate := false
	if currentNode.IsRelayed && (currentNode.Address.String() != newNode.Address.String() || currentNode.Address6.String() != newNode.Address6.String()) {
		relayedUpdate = true
	}
	ifaceDelta := logic.IfaceDelta(&currentNode, newNode)
	aclUpdate := currentNode.DefaultACL != newNode.DefaultACL
	if ifaceDelta && servercfg.Is_EE {
		if err = logic.EnterpriseResetAllPeersFailovers(currentNode.ID, currentNode.Network); err != nil {
			logger.Log(0, "failed to reset failover lists during node update for node", currentNode.ID.String(), currentNode.Network)
		}
	}

	err = logic.UpdateNode(&currentNode, newNode)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to update node info [ %s ] info: %v", nodeid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if relayupdate {
		updatenodes := logic.UpdateRelay(currentNode.Network, currentNode.RelayAddrs, newNode.RelayAddrs)
		if len(updatenodes) > 0 {
			for _, relayedNode := range updatenodes {
				runUpdates(&relayedNode, false)
			}
		}
	}
	if relayedUpdate {
		updateRelay(&currentNode, newNode)
	}
	if servercfg.IsDNSMode() {
		logic.SetDNS()
	}

	apiNode := newNode.ConvertToAPINode()
	logger.Log(1, r.Header.Get("user"), "updated node", currentNode.ID.String(), "on network", currentNode.Network)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
	runUpdates(newNode, ifaceDelta)
	go func(aclUpdate bool, newNode *models.Node) {
		if aclUpdate {
			if err := mq.PublishPeerUpdate(); err != nil {
				logger.Log(0, "error during node ACL update for node", newNode.ID.String())
			}
		}
		if err := mq.PublishReplaceDNS(&currentNode, newNode, host); err != nil {
			logger.Log(1, "failed to publish dns update", err.Error())
		}
	}(aclUpdate, newNode)
}

// swagger:route DELETE /api/nodes/{network}/{nodeid} nodes deleteNode
//
// Delete an individual node.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
func deleteNode(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params
	var params = mux.Vars(r)
	var nodeid = params["nodeid"]
	fromNode := r.Header.Get("requestfrom") == "node"
	node, err := logic.GetNodeByID(nodeid)
	if err != nil {
		if logic.CheckAndRemoveLegacyNode(nodeid) {
			logger.Log(0, "removed legacy node", nodeid)
			logic.ReturnSuccessResponse(w, r, nodeid+" deleted.")
		} else {
			logger.Log(0, "error retrieving node to delete", err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		}
		return
	}
	if r.Header.Get("ismaster") != "yes" {
		username := r.Header.Get("user")
		if username != "" && !doesUserOwnNode(username, params["network"], nodeid) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("user not permitted"), "badrequest"))
			return
		}
	}
	if err := logic.DeleteNode(&node, fromNode); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to delete node"), "internal"))
		return
	}
	logic.ReturnSuccessResponse(w, r, nodeid+" deleted.")
	logger.Log(1, r.Header.Get("user"), "Deleted node", nodeid, "from network", params["network"])
	if !fromNode { // notify node change
		runUpdates(&node, false)
	}
	go func(deletedNode *models.Node, fromNode bool) { // notify of peer change
		var err error
		if fromNode {
			err = mq.PublishDeletedNodePeerUpdate(deletedNode)
		} else {
			err = mq.PublishPeerUpdate()
		}
		if err != nil {
			logger.Log(1, "error publishing peer update ", err.Error())
		}

		host, err := logic.GetHost(node.HostID.String())
		if err != nil {
			logger.Log(1, "failed to retrieve host for node", node.ID.String(), err.Error())
		}
		if err := mq.PublishDNSDelete(&node, host); err != nil {
			logger.Log(1, "error publishing dns update", err.Error())
		}
	}(&node, fromNode)
}

func runUpdates(node *models.Node, ifaceDelta bool) {
	go func() { // don't block http response
		// publish node update if not server
		if err := mq.NodeUpdate(node); err != nil {
			logger.Log(1, "error publishing node update to node", node.ID.String(), err.Error())
		}
	}()
}

func updateRelay(oldnode, newnode *models.Node) {
	relay := logic.FindRelay(oldnode)
	newrelay := relay
	//check if node's address has been updated and if so, update the relayAddrs of the relay node with the updated address of the relayed node
	if oldnode.Address.String() != newnode.Address.String() {
		for i, ip := range newrelay.RelayAddrs {
			if ip == oldnode.Address.IP.String() {
				newrelay.RelayAddrs = append(newrelay.RelayAddrs[:i], relay.RelayAddrs[i+1:]...)
				newrelay.RelayAddrs = append(newrelay.RelayAddrs, newnode.Address.IP.String())
			}
		}
	}
	//check if node's address(v6) has been updated and if so, update the relayAddrs of the relay node with the updated address(v6) of the relayed node
	if oldnode.Address6.String() != newnode.Address6.String() {
		for i, ip := range newrelay.RelayAddrs {
			if ip == oldnode.Address.IP.String() {
				newrelay.RelayAddrs = append(newrelay.RelayAddrs[:i], newrelay.RelayAddrs[i+1:]...)
				newrelay.RelayAddrs = append(newrelay.RelayAddrs, newnode.Address6.IP.String())
			}
		}
	}
	logic.UpdateNode(relay, newrelay)
}

func doesUserOwnNode(username, network, nodeID string) bool {
	u, err := logic.GetUser(username)
	if err != nil {
		return false
	}
	if u.IsAdmin {
		return true
	}

	netUser, err := pro.GetNetworkUser(network, promodels.NetworkUserID(u.UserName))
	if err != nil {
		return false
	}

	if netUser.AccessLevel == pro.NET_ADMIN {
		return true
	}

	return logic.StringSliceContains(netUser.Nodes, nodeID)
}
