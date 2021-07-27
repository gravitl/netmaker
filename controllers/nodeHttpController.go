package controller

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
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
	r.HandleFunc("/api/nodes/{network}/{macaddress}/checkin", authorize(true, "node", http.HandlerFunc(checkIn))).Methods("POST")
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

			//Search DB for node with Mac Address. Ignore pending nodes (they should not be able to authenticate with API untill approved).
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
				tokenString, _ := functions.CreateJWT(authRequest.MacAddress, result.Network)

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
			//TODO: There's probably a better way of dealing with the "master token"/master password. Plz Halp.
			var isAuthorized = false
			var macaddress = ""
			username, networks, isadmin, errN := functions.VerifyUserToken(authToken)
			isnetadmin := isadmin
			if errN == nil && isadmin {
				macaddress = "mastermac"
				isAuthorized = true
				r.Header.Set("ismasterkey", "yes")
			} else {
                                r.Header.Set("ismasterkey", "")
				mac, _, err := functions.VerifyToken(authToken)
				if err != nil {
					errorResponse = models.ErrorResponse{
						Code: http.StatusUnauthorized, Message: "W1R3: Error Verifying Auth Token.",
					}
					returnErrorResponse(w, r, errorResponse)
					return
				}
				macaddress = mac
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
						node, err := functions.GetNodeByMacAddress(params["network"], macaddress)
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
	nodes, err := GetNetworkNodes(networkName)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	//Returns all the nodes in JSON format
	functions.PrintUserLog(r.Header.Get("user"), "fetched nodes on network"+networkName, 2)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(nodes)
}

func GetNetworkNodes(network string) ([]models.Node, error) {
	var nodes []models.Node
	collection, err := database.FetchRecords(database.NODES_TABLE_NAME)
	if err != nil {
		return nodes, err
	}
	for _, value := range collection {

		var node models.Node
		err := json.Unmarshal([]byte(value), &node)
		if err != nil {
			continue
		}
		if node.Network == network {
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

//A separate function to get all nodes, not just nodes for a particular network.
//Not quite sure if this is necessary. Probably necessary based on front end but may want to review after iteration 1 if it's being used or not
func getAllNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	user, err := functions.GetUser(r.Header.Get("user"))
	if err != nil && r.Header.Get("ismasterkey") != "yes" {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	var nodes []models.Node
	if user.IsAdmin  || r.Header.Get("ismasterkey") == "yes" {
		nodes, err = models.GetAllNodes()
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
		tmpNodes, err := GetNetworkNodes(networkName)
		if err != nil {
			continue
		}
		nodes = append(nodes, tmpNodes...)
	}
	return nodes, err
}

//This function get's called when a node "checks in" at check in interval
//Honestly I'm not sure what all it should be doing
//TODO: Implement the necessary stuff, including the below
//Check the last modified of the network
//Check the last modified of the nodes
//Write functions for responding to these two thingies
func checkIn(w http.ResponseWriter, r *http.Request) {

	//TODO: Current thoughts:
	//Dont bother with a networklastmodified
	//Instead, implement a "configupdate" boolean on nodes
	//when there is a network update  that requrires  a config update,  then the node will pull its new config

	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	node, err := CheckIn(params["network"], params["macaddress"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}
func CheckIn(network string, macaddress string) (models.Node, error) {
	var node models.Node

	node, err := GetNode(macaddress, network)
	key, err := functions.GetRecordKey(macaddress, network)
	if err != nil {
		return node, err
	}
	time := time.Now().Unix()
	node.LastCheckIn = time
	data, err := json.Marshal(&node)
	if err != nil {
		return node, err
	}
	err = database.Insert(key, string(data), database.NODES_TABLE_NAME)
	return node, err
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

	network, err := node.GetNetwork()
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	//Check to see if key is valid
	//TODO: Triple inefficient!!! This is the third call to the DB we make for networks
	validKey := functions.IsKeyValid(networkName, node.AccessKey)

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

	node, err = CreateNode(node, networkName)
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

func UncordonNode(network, macaddress string) (models.Node, error) {
	node, err := functions.GetNodeByMacAddress(network, macaddress)
	if err != nil {
		return models.Node{}, err
	}
	node.SetLastModified()
	node.IsPending = "no"
	data, err := json.Marshal(&node)
	if err != nil {
		return node, err
	}
	key, err := functions.GetRecordKey(node.MacAddress, node.Network)
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

func CreateEgressGateway(gateway models.EgressGatewayRequest) (models.Node, error) {
	node, err := functions.GetNodeByMacAddress(gateway.NetID, gateway.NodeID)
	if err != nil {
		return models.Node{}, err
	}
	log.Println("GATEWAY:",gateway)
	log.Println("NODE:",node)
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
	key, err := functions.GetRecordKey(gateway.NodeID, gateway.NetID)
	if err != nil {
		return node, err
	}
	node.PostUp = postUpCmd
	node.PostDown = postDownCmd
	node.SetLastModified()
	nodeData, err := json.Marshal(&node)
	if err != nil {
		return node, err
	}
	err = database.Insert(key, string(nodeData), database.NODES_TABLE_NAME)
	// prepare update model.
	if err != nil {
		return models.Node{}, err
	}
	err = SetNetworkNodesLastModified(gateway.NetID)
	return node, err
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
	functions.PrintUserLog(r.Header.Get("user"), "delete egress gateway "+nodeMac+" on network "+netid, 1)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

func DeleteEgressGateway(network, macaddress string) (models.Node, error) {

	node, err := functions.GetNodeByMacAddress(network, macaddress)
	if err != nil {
		return models.Node{}, err
	}

	node.IsEgressGateway = "no"
	node.EgressGatewayRanges = []string{}
	node.PostUp = ""
	node.PostDown = ""
	node.SetLastModified()
	key, err := functions.GetRecordKey(node.MacAddress, node.Network)
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
	err = SetNetworkNodesLastModified(network)
	if err != nil {
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

func CreateIngressGateway(netid string, macaddress string) (models.Node, error) {

	node, err := functions.GetNodeByMacAddress(netid, macaddress)
	if err != nil {
		return models.Node{}, err
	}

	network, err := functions.GetParentNetwork(netid)
	if err != nil {
		log.Println("Could not find network.")
		return models.Node{}, err
	}
	node.IsIngressGateway = "yes"
	node.IngressGatewayRange = network.AddressRange
	postUpCmd := "iptables -A FORWARD -i " + node.Interface + " -j ACCEPT; iptables -t nat -A POSTROUTING -o " + node.Interface + " -j MASQUERADE"
	postDownCmd := "iptables -D FORWARD -i " + node.Interface + " -j ACCEPT; iptables -t nat -D POSTROUTING -o " + node.Interface + " -j MASQUERADE"
	if node.PostUp != "" {
		if !strings.Contains(node.PostUp, postUpCmd) {
			node.PostUp = node.PostUp + "; " + postUpCmd
		}
	}
	if node.PostDown != "" {
		if !strings.Contains(node.PostDown, postDownCmd) {
			node.PostDown = node.PostDown + "; " + postDownCmd
		}
	}
	node.SetLastModified()

	key, err := functions.GetRecordKey(node.MacAddress, node.Network)
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
	err = SetNetworkNodesLastModified(netid)
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

func DeleteIngressGateway(network, macaddress string) (models.Node, error) {

	node, err := functions.GetNodeByMacAddress(network, macaddress)
	if err != nil {
		return models.Node{}, err
	}
	node.LastModified = time.Now().Unix()
	node.IsIngressGateway = "no"
	key, err := functions.GetRecordKey(node.MacAddress, node.Network)
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
	err = SetNetworkNodesLastModified(network)
	return node, err
}

func updateNode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	var node models.Node
	//start here
	node, err := functions.GetNodeByMacAddress(params["network"], params["macaddress"])
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
	err = node.Update(&newNode)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	if err = SetNetworkNodesLastModified(node.Network); err != nil {
		log.Println(err)
	}
	if servercfg.IsDNSMode() {
		err = SetDNS()
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

	err := DeleteNode(params["macaddress"], params["network"])

	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "Deleted node "+params["macaddress"]+" from network "+params["network"], 1)
	returnSuccessResponse(w, r, params["macaddress"]+" deleted.")
}
