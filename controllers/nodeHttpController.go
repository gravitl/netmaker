package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/crypto/bcrypt"
)

func nodeHandlers(r *mux.Router) {

	r.HandleFunc("/api/nodes", authorize(false, "user", http.HandlerFunc(getAllNodes))).Methods("GET")
	r.HandleFunc("/api/nodes/{network}", authorize(true, "network", http.HandlerFunc(getNetworkNodes))).Methods("GET")
	r.HandleFunc("/api/nodes/{network}/{macaddress}", authorize(true, "node", http.HandlerFunc(getNode))).Methods("GET")
	r.HandleFunc("/api/nodes/{network}/{macaddress}", authorize(true, "node", http.HandlerFunc(updateNode))).Methods("PUT")
	r.HandleFunc("/api/nodes/{network}/{macaddress}", authorize(true, "node", http.HandlerFunc(deleteNode))).Methods("DELETE")
	r.HandleFunc("/api/nodes/{network}/{macaddress}/createrelay", authorize(true, "user", http.HandlerFunc(createRelay))).Methods("POST")
	r.HandleFunc("/api/nodes/{network}/{macaddress}/deleterelay", authorize(true, "user", http.HandlerFunc(deleteRelay))).Methods("DELETE")
	r.HandleFunc("/api/nodes/{network}/{macaddress}/creategateway", authorize(true, "user", http.HandlerFunc(createEgressGateway))).Methods("POST")
	r.HandleFunc("/api/nodes/{network}/{macaddress}/deletegateway", authorize(true, "user", http.HandlerFunc(deleteEgressGateway))).Methods("DELETE")
	r.HandleFunc("/api/nodes/{network}/{macaddress}/createingress", securityCheck(false, http.HandlerFunc(createIngressGateway))).Methods("POST")
	r.HandleFunc("/api/nodes/{network}/{macaddress}/deleteingress", securityCheck(false, http.HandlerFunc(deleteIngressGateway))).Methods("DELETE")
	r.HandleFunc("/api/nodes/{network}/{macaddress}/approve", authorize(true, "user", http.HandlerFunc(uncordonNode))).Methods("POST")
	r.HandleFunc("/api/nodes/{network}", createNode).Methods("POST")
	r.HandleFunc("/api/nodes/adm/{network}/lastmodified", authorize(true, "network", http.HandlerFunc(getLastModified))).Methods("GET")
	r.HandleFunc("/api/nodes/adm/{network}/authenticate", authenticate).Methods("POST")

}

//Node authenticates using its password and retrieves a JWT for authorization.
func authenticate(response http.ResponseWriter, request *http.Request) {

	var params = mux.Vars(request)
	networkname := params["network"]
	//Auth request consists of Mac Address and Password (from node that is authorizing
	//in case of Master, auth is ignored and mac is set to "mastermac"
	var authRequest models.AuthParams
	var result models.Node
	var errorResponse = models.ErrorResponse{
		Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
	}

	//Get password fnd mac rom request
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

			//Search DB for node with Mac Address. Ignore pending nodes (they should not be able to authenticate with API until approved).
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
				if result.MacAddress == authRequest.MacAddress && result.IsPending != "yes" && result.Network == networkname {
					break
				}
			}

			if err != nil {
				errorResponse.Code = http.StatusBadRequest
				errorResponse.Message = err.Error()
				returnErrorResponse(response, request, errorResponse)
				return
			}

			//compare password from request to stored password in database
			//might be able to have a common hash (certificates?) and compare those so that a password isn't passed in in plain text...
			//TODO: Consider a way of hashing the password client side before sending, or using certificates
			err = bcrypt.CompareHashAndPassword([]byte(result.Password), []byte(authRequest.Password))
			if err != nil {
				errorResponse.Code = http.StatusBadRequest
				errorResponse.Message = err.Error()
				returnErrorResponse(response, request, errorResponse)
				return
			} else {
				//Create a new JWT for the node
				tokenString, _ := logic.CreateJWT(authRequest.MacAddress, result.Network)

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
				//Send back the JWT
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

			//This checks if
			//A: the token is the master password
			//B: the token corresponds to a mac address, and if so, which one
			//TODO: There's probably a better way of dealing with the "master token"/master password. Plz Help.
			var isAuthorized = false
			var macaddress = ""
			username, networks, isadmin, errN := logic.VerifyUserToken(authToken)
			isnetadmin := isadmin
			if errN == nil && isadmin {
				macaddress = "mastermac"
				isAuthorized = true
				r.Header.Set("ismasterkey", "yes")
			}
			if !isadmin && params["network"] != "" {
				if functions.SliceContains(networks, params["network"]) {
					isnetadmin = true
				}
			}
			//The mastermac (login with masterkey from config) can do everything!! May be dangerous.
			if macaddress == "mastermac" {
				isAuthorized = true
				r.Header.Set("ismasterkey", "yes")
				//for everyone else, there's poor man's RBAC. The "cases" are defined in the routes in the handlers
				//So each route defines which access network should be allowed to access it
			} else {
				switch authNetwork {
				case "all":
					isAuthorized = true
				case "nodes":
					isAuthorized = (macaddress != "") || isnetadmin
				case "network":
					if isnetadmin {
						isAuthorized = true
					} else {
						node, err := logic.GetNodeByMacAddress(params["network"], macaddress)
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
						isAuthorized = (macaddress == params["macaddress"])
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
	functions.PrintUserLog(r.Header.Get("user"), "fetched nodes on network"+networkName, 2)
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
	functions.PrintUserLog(r.Header.Get("user"), "fetched nodes", 2)
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

	node, err := GetNode(params["macaddress"], params["network"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "fetched node "+params["macaddress"], 2)
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
	network, err := GetNetwork(params["network"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "called last modified", 2)
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

	var node models.Node

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

	//Check to see if key is valid
	//TODO: Triple inefficient!!! This is the third call to the DB we make for networks
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

	node, err = logic.CreateNode(node, networkName)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "created new node "+node.Name+" on network "+node.Network, 1)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

//Takes node out of pending state
//TODO: May want to use cordon/uncordon terminology instead of "ispending".
func uncordonNode(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	node, err := UncordonNode(params["network"], params["macaddress"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "uncordoned node "+node.Name, 1)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode("SUCCESS")
}

// UncordonNode - approves a node to join a network
func UncordonNode(network, macaddress string) (models.Node, error) {
	node, err := logic.GetNodeByMacAddress(network, macaddress)
	if err != nil {
		return models.Node{}, err
	}
	node.SetLastModified()
	node.IsPending = "no"
	node.PullChanges = "yes"
	data, err := json.Marshal(&node)
	if err != nil {
		return node, err
	}
	key, err := logic.GetRecordKey(node.MacAddress, node.Network)
	if err != nil {
		return node, err
	}

	err = database.Insert(key, string(data), database.NODES_TABLE_NAME)
	return node, err
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
	gateway.NodeID = params["macaddress"]
	node, err := CreateEgressGateway(gateway)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "created egress gateway on node "+gateway.NodeID+" on network "+gateway.NetID, 1)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

// CreateEgressGateway - creates an egress gateway
func CreateEgressGateway(gateway models.EgressGatewayRequest) (models.Node, error) {
	node, err := logic.GetNodeByMacAddress(gateway.NetID, gateway.NodeID)
	if node.OS == "windows" || node.OS == "macos" { // add in darwin later
		return models.Node{}, errors.New(node.OS + " is unsupported for egress gateways")
	}
	if err != nil {
		return models.Node{}, err
	}
	err = ValidateEgressGateway(gateway)
	if err != nil {
		return models.Node{}, err
	}
	node.IsEgressGateway = "yes"
	node.EgressGatewayRanges = gateway.Ranges
	postUpCmd := "iptables -A FORWARD -i " + node.Interface + " -j ACCEPT; iptables -t nat -A POSTROUTING -o " + gateway.Interface + " -j MASQUERADE"
	postDownCmd := "iptables -D FORWARD -i " + node.Interface + " -j ACCEPT; iptables -t nat -D POSTROUTING -o " + gateway.Interface + " -j MASQUERADE"
	if gateway.PostUp != "" {
		postUpCmd = gateway.PostUp
	}
	if gateway.PostDown != "" {
		postDownCmd = gateway.PostDown
	}
	if node.PostUp != "" {
		if !strings.Contains(node.PostUp, postUpCmd) {
			postUpCmd = node.PostUp + "; " + postUpCmd
		}
	}
	if node.PostDown != "" {
		if !strings.Contains(node.PostDown, postDownCmd) {
			postDownCmd = node.PostDown + "; " + postDownCmd
		}
	}
	key, err := logic.GetRecordKey(gateway.NodeID, gateway.NetID)
	if err != nil {
		return node, err
	}
	node.PostUp = postUpCmd
	node.PostDown = postDownCmd
	node.SetLastModified()
	node.PullChanges = "yes"
	nodeData, err := json.Marshal(&node)
	if err != nil {
		return node, err
	}
	if err = database.Insert(key, string(nodeData), database.NODES_TABLE_NAME); err != nil {
		return models.Node{}, err
	}
	if err = functions.NetworkNodesUpdatePullChanges(node.Network); err != nil {
		return models.Node{}, err
	}
	return node, nil
}

func ValidateEgressGateway(gateway models.EgressGatewayRequest) error {
	var err error
	//isIp := functions.IsIpCIDR(gateway.RangeString)
	empty := len(gateway.Ranges) == 0
	if empty {
		err = errors.New("IP Ranges Cannot Be Empty")
	}
	empty = gateway.Interface == ""
	if empty {
		err = errors.New("Interface cannot be empty")
	}
	return err
}

func deleteEgressGateway(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	nodeMac := params["macaddress"]
	netid := params["network"]
	node, err := DeleteEgressGateway(netid, nodeMac)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "deleted egress gateway "+nodeMac+" on network "+netid, 1)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

// DeleteEgressGateway - deletes egress from node
func DeleteEgressGateway(network, macaddress string) (models.Node, error) {

	node, err := logic.GetNodeByMacAddress(network, macaddress)
	if err != nil {
		return models.Node{}, err
	}

	node.IsEgressGateway = "no"
	node.EgressGatewayRanges = []string{}
	node.PostUp = ""
	node.PostDown = ""
	if node.IsIngressGateway == "yes" { // check if node is still an ingress gateway before completely deleting postdown/up rules
		node.PostUp = "iptables -A FORWARD -i " + node.Interface + " -j ACCEPT; iptables -t nat -A POSTROUTING -o " + node.Interface + " -j MASQUERADE"
		node.PostDown = "iptables -D FORWARD -i " + node.Interface + " -j ACCEPT; iptables -t nat -D POSTROUTING -o " + node.Interface + " -j MASQUERADE"
	}
	node.SetLastModified()
	node.PullChanges = "yes"
	key, err := logic.GetRecordKey(node.MacAddress, node.Network)
	if err != nil {
		return models.Node{}, err
	}
	data, err := json.Marshal(&node)
	if err != nil {
		return models.Node{}, err
	}
	if err = database.Insert(key, string(data), database.NODES_TABLE_NAME); err != nil {
		return models.Node{}, err
	}
	if err = functions.NetworkNodesUpdatePullChanges(network); err != nil {
		return models.Node{}, err
	}
	return node, nil
}

// == INGRESS ==
func createIngressGateway(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	nodeMac := params["macaddress"]
	netid := params["network"]
	node, err := CreateIngressGateway(netid, nodeMac)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "created ingress gateway on node "+nodeMac+" on network "+netid, 1)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

// CreateIngressGateway - creates an ingress gateway
func CreateIngressGateway(netid string, macaddress string) (models.Node, error) {

	node, err := logic.GetNodeByMacAddress(netid, macaddress)
	if node.OS == "windows" || node.OS == "macos" { // add in darwin later
		return models.Node{}, errors.New(node.OS + " is unsupported for ingress gateways")
	}

	if err != nil {
		return models.Node{}, err
	}

	network, err := logic.GetParentNetwork(netid)
	if err != nil {
		return models.Node{}, err
	}
	node.IsIngressGateway = "yes"
	node.IngressGatewayRange = network.AddressRange
	postUpCmd := "iptables -A FORWARD -i " + node.Interface + " -j ACCEPT; iptables -t nat -A POSTROUTING -o " + node.Interface + " -j MASQUERADE"
	postDownCmd := "iptables -D FORWARD -i " + node.Interface + " -j ACCEPT; iptables -t nat -D POSTROUTING -o " + node.Interface + " -j MASQUERADE"
	if node.PostUp != "" {
		if !strings.Contains(node.PostUp, postUpCmd) {
			postUpCmd = node.PostUp + "; " + postUpCmd
		}
	}
	if node.PostDown != "" {
		if !strings.Contains(node.PostDown, postDownCmd) {
			postDownCmd = node.PostDown + "; " + postDownCmd
		}
	}
	node.SetLastModified()
	node.PostUp = postUpCmd
	node.PostDown = postDownCmd
	node.PullChanges = "yes"
	node.UDPHolePunch = "no"
	key, err := logic.GetRecordKey(node.MacAddress, node.Network)
	if err != nil {
		return models.Node{}, err
	}
	data, err := json.Marshal(&node)
	if err != nil {
		return models.Node{}, err
	}
	err = database.Insert(key, string(data), database.NODES_TABLE_NAME)
	if err != nil {
		return models.Node{}, err
	}
	err = logic.SetNetworkNodesLastModified(netid)
	return node, err
}

func deleteIngressGateway(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	nodeMac := params["macaddress"]
	node, err := DeleteIngressGateway(params["network"], nodeMac)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "deleted ingress gateway"+nodeMac, 1)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

// DeleteIngressGateway - deletes an ingress gateway
func DeleteIngressGateway(networkName string, macaddress string) (models.Node, error) {

	node, err := logic.GetNodeByMacAddress(networkName, macaddress)
	if err != nil {
		return models.Node{}, err
	}
	network, err := logic.GetParentNetwork(networkName)
	if err != nil {
		return models.Node{}, err
	}
	// delete ext clients belonging to ingress gateway
	if err = DeleteGatewayExtClients(macaddress, networkName); err != nil {
		return models.Node{}, err
	}

	node.UDPHolePunch = network.DefaultUDPHolePunch
	node.LastModified = time.Now().Unix()
	node.IsIngressGateway = "no"
	node.IngressGatewayRange = ""
	node.PullChanges = "yes"

	key, err := logic.GetRecordKey(node.MacAddress, node.Network)
	if err != nil {
		return models.Node{}, err
	}
	data, err := json.Marshal(&node)
	if err != nil {
		return models.Node{}, err
	}
	err = database.Insert(key, string(data), database.NODES_TABLE_NAME)
	if err != nil {
		return models.Node{}, err
	}
	err = logic.SetNetworkNodesLastModified(networkName)
	return node, err
}

func updateNode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	var node models.Node
	//start here
	node, err := logic.GetNodeByMacAddress(params["network"], params["macaddress"])
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
	err = logic.UpdateNode(&node, &newNode)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	if relayupdate {
		UpdateRelay(node.Network, node.RelayAddrs, newNode.RelayAddrs)
		if err = functions.NetworkNodesUpdatePullChanges(node.Network); err != nil {
			functions.PrintUserLog("netmaker", "error setting relay updates: "+err.Error(), 1)
		}
	}

	if servercfg.IsDNSMode() {
		err = logic.SetDNS()
	}
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "updated node "+node.MacAddress+" on network "+node.Network, 1)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newNode)
}

//Delete a node
//Pretty straightforward
func deleteNode(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params
	var params = mux.Vars(r)

	err := DeleteNode(params["macaddress"]+"###"+params["network"], false)

	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "Deleted node "+params["macaddress"]+" from network "+params["network"], 1)
	returnSuccessResponse(w, r, params["macaddress"]+" deleted.")
}
