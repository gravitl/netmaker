package controller

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/crypto/bcrypt"
)

func nodeHandlers(r *mux.Router) {

	r.HandleFunc("/api/nodes", authorize(false, "user", http.HandlerFunc(getAllNodes))).Methods("GET")
	r.HandleFunc("/api/nodes/{network}", authorize(true, "network", http.HandlerFunc(getNetworkNodes))).Methods("GET")
	r.HandleFunc("/api/nodes/{network}/{nodeid}", authorize(true, "node", http.HandlerFunc(getNode))).Methods("GET")
	r.HandleFunc("/api/nodes/{network}/{nodeid}", authorize(true, "node", http.HandlerFunc(updateNode))).Methods("PUT")
	r.HandleFunc("/api/nodes/{network}/{nodeid}", authorize(true, "node", http.HandlerFunc(deleteNode))).Methods("DELETE")
	r.HandleFunc("/api/nodes/{network}/{nodeid}/createrelay", authorize(true, "user", http.HandlerFunc(createRelay))).Methods("POST")
	r.HandleFunc("/api/nodes/{network}/{nodeid}/deleterelay", authorize(true, "user", http.HandlerFunc(deleteRelay))).Methods("DELETE")
	r.HandleFunc("/api/nodes/{network}/{nodeid}/creategateway", authorize(true, "user", http.HandlerFunc(createEgressGateway))).Methods("POST")
	r.HandleFunc("/api/nodes/{network}/{nodeid}/deletegateway", authorize(true, "user", http.HandlerFunc(deleteEgressGateway))).Methods("DELETE")
	r.HandleFunc("/api/nodes/{network}/{nodeid}/createingress", securityCheck(false, http.HandlerFunc(createIngressGateway))).Methods("POST")
	r.HandleFunc("/api/nodes/{network}/{nodeid}/deleteingress", securityCheck(false, http.HandlerFunc(deleteIngressGateway))).Methods("DELETE")
	r.HandleFunc("/api/nodes/{network}/{nodeid}/approve", authorize(true, "user", http.HandlerFunc(uncordonNode))).Methods("POST")
	// r.HandleFunc("/api/nodes/{network}", createNode).Methods("POST")
	r.HandleFunc("/api/nodes/adm/{network}/lastmodified", authorize(true, "network", http.HandlerFunc(getLastModified))).Methods("GET")
	r.HandleFunc("/api/nodes/adm/{network}/authenticate", authenticate).Methods("POST")

}

func authenticate(response http.ResponseWriter, request *http.Request) {

	var params = mux.Vars(request)
	networkname := params["network"]

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
		returnErrorResponse(response, request, errorResponse)
		return
	} else {
		errorResponse.Code = http.StatusBadRequest
		if authRequest.MacAddress == "" {
			errorResponse.Message = "W1R3: MacAddress can't be empty"
			returnErrorResponse(response, request, errorResponse)
			return
		} else if authRequest.Password == "" {
			errorResponse.Message = "W1R3: Password can't be empty"
			returnErrorResponse(response, request, errorResponse)
			return
		} else {

			collection, err := database.FetchRecords(database.NODES_TABLE_NAME)
			if err != nil {
				errorResponse.Code = http.StatusBadRequest
				errorResponse.Message = err.Error()
				returnErrorResponse(response, request, errorResponse)
				return
			}
			for _, value := range collection {
				if err := json.Unmarshal([]byte(value), &result); err != nil {
					continue
				}
				if (result.ID == authRequest.ID || result.MacAddress == authRequest.MacAddress) && result.IsPending != "yes" && result.Network == networkname {
					break
				}
			}

			if err != nil {
				errorResponse.Code = http.StatusBadRequest
				errorResponse.Message = err.Error()
				returnErrorResponse(response, request, errorResponse)
				return
			}

			err = bcrypt.CompareHashAndPassword([]byte(result.Password), []byte(authRequest.Password))
			if err != nil {
				errorResponse.Code = http.StatusBadRequest
				errorResponse.Message = err.Error()
				returnErrorResponse(response, request, errorResponse)
				return
			} else {
				tokenString, _ := logic.CreateJWT(authRequest.ID, authRequest.MacAddress, result.Network)

				if tokenString == "" {
					errorResponse.Code = http.StatusBadRequest
					errorResponse.Message = "Could not create Token"
					returnErrorResponse(response, request, errorResponse)
					return
				}

				var successResponse = models.SuccessResponse{
					Code:    http.StatusOK,
					Message: "W1R3: Device " + authRequest.MacAddress + " Authorized",
					Response: models.SuccessfulLoginResponse{
						AuthToken:  tokenString,
						MacAddress: authRequest.MacAddress,
					},
				}
				successJSONResponse, jsonError := json.Marshal(successResponse)

				if jsonError != nil {
					errorResponse.Code = http.StatusBadRequest
					errorResponse.Message = err.Error()
					returnErrorResponse(response, request, errorResponse)
					return
				}
				response.WriteHeader(http.StatusOK)
				response.Header().Set("Content-Type", "application/json")
				response.Write(successJSONResponse)
			}
		}
	}
}

//The middleware for most requests to the API
//They all pass  through here first
//This will validate the JWT (or check for master token)
//This will also check against the authNetwork and make sure the node should be accessing that endpoint,
//even if it's technically ok
//This is kind of a poor man's RBAC. There's probably a better/smarter way.
//TODO: Consider better RBAC implementations
func authorize(networkCheck bool, authNetwork string, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
		}

		var params = mux.Vars(r)

		networkexists, _ := functions.NetworkExists(params["network"])
		//check that the request is for a valid network
		//if (networkCheck && !networkexists) || err != nil {
		if networkCheck && !networkexists {
			errorResponse = models.ErrorResponse{
				Code: http.StatusNotFound, Message: "W1R3: This network does not exist. ",
			}
			returnErrorResponse(w, r, errorResponse)
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
				errorResponse = models.ErrorResponse{
					Code: http.StatusUnauthorized, Message: "W1R3: Missing Auth Token.",
				}
				returnErrorResponse(w, r, errorResponse)
				return
			}

			var isAuthorized = false
			var nodeID = ""
			username, networks, isadmin, errN := logic.VerifyUserToken(authToken)
			isnetadmin := isadmin
			if errN == nil && isadmin {
				nodeID = "mastermac"
				isAuthorized = true
				r.Header.Set("ismasterkey", "yes")
			}
			if !isadmin && params["network"] != "" {
				if functions.SliceContains(networks, params["network"]) {
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
							errorResponse = models.ErrorResponse{
								Code: http.StatusUnauthorized, Message: "W1R3: Missing Auth Token.",
							}
							returnErrorResponse(w, r, errorResponse)
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
				case "user":
					isAuthorized = true
				default:
					isAuthorized = false
				}
			}
			if !isAuthorized {
				errorResponse = models.ErrorResponse{
					Code: http.StatusUnauthorized, Message: "W1R3: You are unauthorized to access this endpoint.",
				}
				returnErrorResponse(w, r, errorResponse)
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

//Gets all nodes associated with network, including pending nodes
func getNetworkNodes(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var nodes []models.Node
	var params = mux.Vars(r)
	networkName := params["network"]
	nodes, err := logic.GetNetworkNodes(networkName)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	//Returns all the nodes in JSON format
	logger.Log(2, r.Header.Get("user"), "fetched nodes on network", networkName)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(nodes)
}

//A separate function to get all nodes, not just nodes for a particular network.
//Not quite sure if this is necessary. Probably necessary based on front end but may want to review after iteration 1 if it's being used or not
func getAllNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user, err := logic.GetUser(r.Header.Get("user"))
	if err != nil && r.Header.Get("ismasterkey") != "yes" {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	var nodes []models.Node
	if user.IsAdmin || r.Header.Get("ismasterkey") == "yes" {
		nodes, err = logic.GetAllNodes()
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		}
	} else {
		nodes, err = getUsersNodes(user)
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		}
	}
	//Return all the nodes in JSON format
	logger.Log(2, r.Header.Get("user"), "fetched nodes")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(nodes)
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

//Get an individual node. Nothin fancy here folks.
func getNode(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	node, err := logic.GetNodeByID(params["nodeid"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(2, r.Header.Get("user"), "fetched node", params["nodeid"])
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

//Get the time that a network of nodes was last modified.
//TODO: This needs to be refactored
//Potential way to do this: On UpdateNode, set a new field for "LastModified"
//If we go with the existing way, we need to at least set network.NodesLastModified on UpdateNode
func getLastModified(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	network, err := logic.GetNetwork(params["network"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(2, r.Header.Get("user"), "called last modified")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(network.NodesLastModified)
}

func createNode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	var errorResponse = models.ErrorResponse{
		Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
	}
	networkName := params["network"]
	networkexists, err := functions.NetworkExists(networkName)

	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	} else if !networkexists {
		errorResponse = models.ErrorResponse{
			Code: http.StatusNotFound, Message: "W1R3: Network does not exist! ",
		}
		returnErrorResponse(w, r, errorResponse)
		return
	}

	var node = models.Node{}

	//get node from body of request
	err = json.NewDecoder(r.Body).Decode(&node)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	node.Network = networkName

	network, err := logic.GetNetworkByNode(&node)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	validKey := logic.IsKeyValid(networkName, node.AccessKey)

	if !validKey {
		//Check to see if network will allow manual sign up
		//may want to switch this up with the valid key check and avoid a DB call that way.
		if network.AllowManualSignUp == "yes" {
			node.IsPending = "yes"
		} else {
			errorResponse = models.ErrorResponse{
				Code: http.StatusUnauthorized, Message: "W1R3: Key invalid, or none provided.",
			}
			returnErrorResponse(w, r, errorResponse)
			return
		}
	}

	err = logic.CreateNode(&node)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "created new node", node.Name, "on network", node.Network)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

//Takes node out of pending state
//TODO: May want to use cordon/uncordon terminology instead of "ispending".
func uncordonNode(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	node, err := logic.UncordonNode(params["nodeid"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "uncordoned node", node.Name)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode("SUCCESS")
}

func createEgressGateway(w http.ResponseWriter, r *http.Request) {
	var gateway models.EgressGatewayRequest
	var params = mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	err := json.NewDecoder(r.Body).Decode(&gateway)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	gateway.NetID = params["network"]
	gateway.NodeID = params["nodeid"]
	node, err := logic.CreateEgressGateway(gateway)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "created egress gateway on node", gateway.NodeID, "on network", gateway.NetID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

func deleteEgressGateway(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	nodeid := params["nodeid"]
	netid := params["network"]
	node, err := logic.DeleteEgressGateway(netid, nodeid)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "deleted egress gateway", nodeid, "on network", netid)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

// == INGRESS ==

func createIngressGateway(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	nodeid := params["nodeid"]
	netid := params["network"]
	node, err := logic.CreateIngressGateway(netid, nodeid)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "created ingress gateway on node", nodeid, "on network", netid)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

func deleteIngressGateway(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	nodeid := params["nodeid"]
	node, err := logic.DeleteIngressGateway(params["network"], nodeid)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "deleted ingress gateway", nodeid)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

func updateNode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	var node models.Node
	//start here
	node, err := logic.GetNodeByID(params["nodeid"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	var newNode models.Node
	// we decode our body request params
	err = json.NewDecoder(r.Body).Decode(&newNode)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	newNode.PullChanges = "yes"
	relayupdate := false
	if node.IsRelay == "yes" && len(newNode.RelayAddrs) > 0 {
		if len(newNode.RelayAddrs) != len(node.RelayAddrs) {
			relayupdate = true
		} else {
			for i, addr := range newNode.RelayAddrs {
				if addr != node.RelayAddrs[i] {
					relayupdate = true
				}
			}
		}
	}

	if !servercfg.GetRce() {
		newNode.PostDown = node.PostDown
		newNode.PostUp = node.PostUp
	}

	err = logic.UpdateNode(&node, &newNode)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	if relayupdate {
		logic.UpdateRelay(node.Network, node.RelayAddrs, newNode.RelayAddrs)
		if err = logic.NetworkNodesUpdatePullChanges(node.Network); err != nil {
			logger.Log(1, "error setting relay updates:", err.Error())
		}
	}

	if servercfg.IsDNSMode() {
		err = logic.SetDNS()
	}
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	logger.Log(1, r.Header.Get("user"), "updated node", node.MacAddress, "on network", node.Network)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newNode)
}

func deleteNode(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params
	var params = mux.Vars(r)
	var nodeid = params["nodeid"]
	var node, err = logic.GetNodeByID(nodeid)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	err = logic.DeleteNodeByID(&node, false)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	logger.Log(1, r.Header.Get("user"), "Deleted node", nodeid, "from network", params["network"])
	returnSuccessResponse(w, r, nodeid+" deleted.")
}
