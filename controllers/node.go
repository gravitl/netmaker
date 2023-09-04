package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/slog"
)

var hostIDHeader = "host-id"

func nodeHandlers(r *mux.Router) {

	r.HandleFunc("/api/nodes", Authorize(false, false, "user", http.HandlerFunc(getAllNodes))).Methods(http.MethodGet)
	r.HandleFunc("/api/nodes/{network}", Authorize(false, true, "network", http.HandlerFunc(getNetworkNodes))).Methods(http.MethodGet)
	r.HandleFunc("/api/nodes/{network}/{nodeid}", Authorize(true, true, "node", http.HandlerFunc(getNode))).Methods(http.MethodGet)
	r.HandleFunc("/api/nodes/{network}/{nodeid}", logic.SecurityCheck(true, http.HandlerFunc(updateNode))).Methods(http.MethodPut)
	r.HandleFunc("/api/nodes/{network}/{nodeid}", Authorize(true, true, "node", http.HandlerFunc(deleteNode))).Methods(http.MethodDelete)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/creategateway", logic.SecurityCheck(true, checkFreeTierLimits(limitChoiceEgress, http.HandlerFunc(createEgressGateway)))).Methods(http.MethodPost)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/deletegateway", logic.SecurityCheck(true, http.HandlerFunc(deleteEgressGateway))).Methods(http.MethodDelete)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/createingress", logic.SecurityCheck(true, checkFreeTierLimits(limitChoiceIngress, http.HandlerFunc(createIngressGateway)))).Methods(http.MethodPost)
	r.HandleFunc("/api/nodes/{network}/{nodeid}/deleteingress", logic.SecurityCheck(true, http.HandlerFunc(deleteIngressGateway))).Methods(http.MethodDelete)
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
func Authorize(hostAllowed, networkCheck bool, authNetwork string, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusForbidden, Message: logic.Forbidden_Msg,
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
			username, issuperadmin, isadmin, errN := logic.VerifyUserToken(authToken)
			if errN != nil {
				logic.ReturnErrorResponse(w, r, logic.FormatError(errN, logic.Unauthorized_Msg))
				return
			}

			isnetadmin := issuperadmin || isadmin
			if errN == nil && (issuperadmin || isadmin) {
				nodeID = "mastermac"
				isAuthorized = true
				r.Header.Set("ismasterkey", "yes")
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
	}
	// return all the nodes in JSON/API format
	apiNodes := logic.GetAllNodesAPI(nodes[:])
	logger.Log(3, r.Header.Get("user"), "fetched all nodes they have access to")
	logic.SortApiNodes(apiNodes[:])
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNodes)
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

	node, err := validateParams(nodeid, params["network"])
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	host, err := logic.GetHost(node.HostID.String())
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("error fetching host for node [ %s ] info: %v", nodeid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	allNodes, err := logic.GetAllNodes()
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("error fetching wg peers config for host [ %s ]: %v", host.ID.String(), err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	hostPeerUpdate, err := logic.GetPeerUpdateForHost(node.Network, host, allNodes, nil, nil)
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

	if servercfg.IsPro && nodeRequest {
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
	node, err := validateParams(params["nodeid"], params["network"])
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "bad request"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewDecoder(r.Body).Decode(&gateway); err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	gateway.NetID = params["network"]
	gateway.NodeID = params["nodeid"]
	node, err = logic.CreateEgressGateway(gateway)
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
	mq.RunUpdates(&node, true)
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
	node, err := validateParams(nodeid, netid)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "bad request"))
		return
	}
	node, err = logic.DeleteEgressGateway(netid, nodeid)
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
	mq.RunUpdates(&node, true)
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
	node, err := validateParams(nodeid, netid)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "bad request"))
		return
	}
	var request models.IngressRequest
	json.NewDecoder(r.Body).Decode(&request)
	node, err = logic.CreateIngressGateway(netid, nodeid, request)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to create ingress gateway on node [%s] on network [%s]: %v",
				nodeid, netid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	if servercfg.IsPro && request.Failover {
		if err = logic.EnterpriseResetFailoverFunc(node.Network); err != nil {
			logger.Log(1, "failed to reset failover list during failover create", node.ID.String(), node.Network)
		}
	}

	apiNode := node.ConvertToAPINode()
	logger.Log(1, r.Header.Get("user"), "created ingress gateway on node", nodeid, "on network", netid)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)

	mq.RunUpdates(&node, true)
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
	node, err := validateParams(nodeid, netid)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "bad request"))
		return
	}
	node, wasFailover, removedClients, err := logic.DeleteIngressGateway(nodeid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to delete ingress gateway on node [%s] on network [%s]: %v",
				nodeid, netid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	if servercfg.IsPro && wasFailover {
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
			allNodes, err := logic.GetAllNodes()
			if err != nil {
				return
			}
			go mq.PublishSingleHostPeerUpdate(
				host,
				allNodes,
				nil,
				removedClients[:],
			)
		}
	}

	mq.RunUpdates(&node, true)
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
	currentNode, err := validateParams(nodeid, params["network"])
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "bad request"))
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
	relayUpdate := logic.RelayUpdates(&currentNode, newNode)
	host, err := logic.GetHost(newNode.HostID.String())
	if err != nil {
		logger.Log(0, r.Header.Get("user"),
			fmt.Sprintf("failed to get host for node  [ %s ] info: %v", nodeid, err))
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	ifaceDelta := logic.IfaceDelta(&currentNode, newNode)
	aclUpdate := currentNode.DefaultACL != newNode.DefaultACL
	if ifaceDelta && servercfg.IsPro {
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
	if relayUpdate {
		logic.UpdateRelayed(&currentNode, newNode)
	}
	if servercfg.IsDNSMode() {
		logic.SetDNS()
	}

	apiNode := newNode.ConvertToAPINode()
	logger.Log(1, r.Header.Get("user"), "updated node", currentNode.ID.String(), "on network", currentNode.Network)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiNode)
	mq.RunUpdates(newNode, ifaceDelta)
	go func(aclUpdate, relayupdate bool, newNode *models.Node) {
		if aclUpdate || relayupdate {
			if err := mq.PublishPeerUpdate(); err != nil {
				logger.Log(0, "error during node ACL update for node", newNode.ID.String())
			}
		}
		if err := mq.PublishReplaceDNS(&currentNode, newNode, host); err != nil {
			logger.Log(1, "failed to publish dns update", err.Error())
		}
	}(aclUpdate, relayUpdate, newNode)
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
	node, err := validateParams(nodeid, params["network"])
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "bad request"))
		return
	}
	forceDelete := r.URL.Query().Get("force") == "true"
	fromNode := r.Header.Get("requestfrom") == "node"
	if node.IsRelayed {
		// cleanup node from relayednodes on relay node
		relayNode, err := logic.GetNodeByID(node.RelayedBy)
		if err == nil {
			relayedNodes := []string{}
			for _, relayedNodeID := range relayNode.RelayedNodes {
				if relayedNodeID == node.ID.String() {
					continue
				}
				relayedNodes = append(relayedNodes, relayedNodeID)
			}
			relayNode.RelayedNodes = relayedNodes
			logic.UpsertNode(&relayNode)
		}
	}
	if node.IsRelay {
		// unset all the relayed nodes
		logic.SetRelayedNodes(false, node.ID.String(), node.RelayedNodes)
	}
	if node.IsIngressGateway {
		// delete ext clients belonging to ingress gatewa
		go func(node models.Node) {
			if err = logic.DeleteGatewayExtClients(node.ID.String(), node.Network); err != nil {
				slog.Error("failed to delete extclients", "gatewayid", node.ID.String(), "network", node.Network, "error", err.Error())
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, "bad request"))
				return
			}
		}(node)

	}

	purge := forceDelete || fromNode
	if err := logic.DeleteNode(&node, purge); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to delete node"), "internal"))
		return
	}

	logic.ReturnSuccessResponse(w, r, nodeid+" deleted.")
	logger.Log(1, r.Header.Get("user"), "Deleted node", nodeid, "from network", params["network"])
	if !fromNode { // notify node change
		mq.RunUpdates(&node, false)
	}
	go func() { // notify of peer change
		var err error
		err = mq.PublishDeletedNodePeerUpdate(&node)
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
	}()
}

func validateParams(nodeid, netid string) (models.Node, error) {
	node, err := logic.GetNodeByID(nodeid)
	if err != nil {
		slog.Error("error fetching node", "node", nodeid, "error", err.Error())
		return node, fmt.Errorf("error fetching node during parameter validation: %v", err)
	}
	if node.Network != netid {
		slog.Error("network url param does not match node id", "url nodeid", netid, "node", node.Network)
		return node, fmt.Errorf("network url param does not match node network")
	}
	return node, nil
}
