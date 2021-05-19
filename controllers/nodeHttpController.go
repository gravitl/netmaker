package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mongoconn"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

func nodeHandlers(r *mux.Router) {

	r.HandleFunc("/api/nodes", authorize(false, "master", http.HandlerFunc(getAllNodes))).Methods("GET")
	r.HandleFunc("/api/nodes/{network}", authorize(true, "network", http.HandlerFunc(getNetworkNodes))).Methods("GET")
	r.HandleFunc("/api/nodes/{network}/{macaddress}", authorize(true, "node", http.HandlerFunc(getNode))).Methods("GET")
	r.HandleFunc("/api/nodes/{network}/{macaddress}", authorize(true, "node", http.HandlerFunc(updateNode))).Methods("PUT")
	r.HandleFunc("/api/nodes/{network}/{macaddress}", authorize(true, "node", http.HandlerFunc(deleteNode))).Methods("DELETE")
	r.HandleFunc("/api/nodes/{network}/{macaddress}/checkin", authorize(true, "node", http.HandlerFunc(checkIn))).Methods("POST")
	r.HandleFunc("/api/nodes/{network}/{macaddress}/creategateway", authorize(true, "master", http.HandlerFunc(createEgressGateway))).Methods("POST")
	r.HandleFunc("/api/nodes/{network}/{macaddress}/deletegateway", authorize(true, "master", http.HandlerFunc(deleteEgressGateway))).Methods("DELETE")
	r.HandleFunc("/api/nodes/{network}/{macaddress}/createingress", securityCheck(http.HandlerFunc(createIngressGateway))).Methods("POST")
	r.HandleFunc("/api/nodes/{network}/{macaddress}/deleteingress", securityCheck(http.HandlerFunc(deleteIngressGateway))).Methods("DELETE")
	r.HandleFunc("/api/nodes/{network}/{macaddress}/approve", authorize(true, "master", http.HandlerFunc(uncordonNode))).Methods("POST")
	r.HandleFunc("/api/nodes/{network}", createNode).Methods("POST")
	r.HandleFunc("/api/nodes/adm/{network}/lastmodified", authorize(true, "network", http.HandlerFunc(getLastModified))).Methods("GET")
	r.HandleFunc("/api/nodes/adm/{network}/authenticate", authenticate).Methods("POST")

}

//Node authenticates using its password and retrieves a JWT for authorization.
func authenticate(response http.ResponseWriter, request *http.Request) {

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
			collection := mongoconn.Client.Database("netmaker").Collection("nodes")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			var err = collection.FindOne(ctx, bson.M{"macaddress": authRequest.MacAddress, "ispending": false}).Decode(&result)

			defer cancel()

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
			macaddress, _, err := functions.VerifyToken(authToken)
			if err != nil {
				errorResponse = models.ErrorResponse{
					Code: http.StatusUnauthorized, Message: "W1R3: Error Verifying Auth Token.",
				}
				returnErrorResponse(w, r, errorResponse)
				return
			}

			var isAuthorized = false

			//The mastermac (login with masterkey from config) can do everything!! May be dangerous.
			if macaddress == "mastermac" {
				isAuthorized = true

				//for everyone else, there's poor man's RBAC. The "cases" are defined in the routes in the handlers
				//So each route defines which access network should be allowed to access it
			} else {
				switch authNetwork {
				case "all":
					isAuthorized = true
				case "nodes":
					isAuthorized = (macaddress != "")
				case "network":
					node, err := functions.GetNodeByMacAddress(params["network"], macaddress)
					if err != nil {
						errorResponse = models.ErrorResponse{
							Code: http.StatusUnauthorized, Message: "W1R3: Missing Auth Token.",
						}
						returnErrorResponse(w, r, errorResponse)
						return
					}
					isAuthorized = (node.Network == params["network"])
				case "node":
					isAuthorized = (macaddress == params["macaddress"])
				case "master":
					isAuthorized = (macaddress == "mastermac")
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
				next.ServeHTTP(w, r)
			}
		}
	}
}

//Gets all nodes associated with network, including pending nodes
func getNetworkNodes(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var nodes []models.ReturnNode
	var params = mux.Vars(r)
	nodes, err := GetNetworkNodes(params["network"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	//Returns all the nodes in JSON format
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(nodes)
}

func GetNetworkNodes(network string) ([]models.ReturnNode, error) {
	var nodes []models.ReturnNode
	collection := mongoconn.Client.Database("netmaker").Collection("nodes")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	filter := bson.M{"network": network}
	//Filtering out the ID field cuz Dillon doesn't like it. May want to filter out other fields in the future
	cur, err := collection.Find(ctx, filter, options.Find().SetProjection(bson.M{"_id": 0}))
	if err != nil {
		return []models.ReturnNode{}, err
	}
	defer cancel()
	for cur.Next(context.TODO()) {
		//Using a different model for the ReturnNode (other than regular node).
		//Either we should do this for ALL structs (so Networks and Keys)
		//OR we should just use the original struct
		//My preference is to make some new return structs
		//TODO: Think about this. Not an immediate concern. Just need to get some consistency eventually
		var node models.ReturnNode
		err := cur.Decode(&node)
		if err != nil {
			return []models.ReturnNode{}, err
		}
		// add item our array of nodes
		nodes = append(nodes, node)
	}
	//TODO: Another fatal error we should take care of.
	if err := cur.Err(); err != nil {
		return []models.ReturnNode{}, err
	}
	return nodes, nil
}

//A separate function to get all nodes, not just nodes for a particular network.
//Not quite sure if this is necessary. Probably necessary based on front end but may want to review after iteration 1 if it's being used or not
func getAllNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	nodes, err := functions.GetAllNodes()
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	//Return all the nodes in JSON format
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(nodes)
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
func CheckIn(network, macaddress string) (models.Node, error) {
	var node models.Node

	//Retrieves node with DB Call which is inefficient. Let's just get the time and set it.
	//node = functions.GetNodeByMacAddress(params["network"], params["macaddress"])
	collection := mongoconn.Client.Database("netmaker").Collection("nodes")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	filter := bson.M{"macaddress": macaddress, "network": network}
	//old code was inefficient, this is all we need.
	time := time.Now().Unix()
	//node.SetLastCheckIn()
	// prepare update model with new time
	update := bson.D{
		{"$set", bson.D{
			{"lastcheckin", time},
		}},
	}
	err := collection.FindOneAndUpdate(ctx, filter, update).Decode(&node)
	defer cancel()
	if err != nil {
		return models.Node{}, err
	}
	//TODO: check node last modified vs network last modified
	//Get Updated node to return
	node, err = GetNode(macaddress, network)
	if err != nil {
		return models.Node{}, err
	}
	return node, nil
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
	network, err := GetLastModified(params["network"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(network.NodesLastModified)
}

func GetLastModified(network string) (models.Network, error) {
	var net models.Network
	collection := mongoconn.Client.Database("netmaker").Collection("networks")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	filter := bson.M{"netid": network}
	err := collection.FindOne(ctx, filter).Decode(&net)
	defer cancel()
	if err != nil {
		fmt.Println(err)
		return models.Network{}, err
	}
	return net, nil
}

//This one's a doozy
//To create a node
//Must have valid key and be unique
func createNode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	var errorResponse = models.ErrorResponse{
		Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
	}

	networkName := params["network"]

	//Check if network exists  first
	//TODO: This is inefficient. Let's find a better way.
	//Just a few rows down we grab the network anyway
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
		if *network.AllowManualSignUp {
			node.IsPending = true
		} else {
			errorResponse = models.ErrorResponse{
				Code: http.StatusUnauthorized, Message: "W1R3: Key invalid, or none provided.",
			}
			returnErrorResponse(w, r, errorResponse)
			return
		}
	}

	err = ValidateNodeCreate(networkName, node)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}

	node, err = CreateNode(node, networkName)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
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
	fmt.Println("Node " + node.Name + " uncordoned.")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode("SUCCESS")
}

func UncordonNode(network, macaddress string) (models.Node, error) {
	node, err := functions.GetNodeByMacAddress(network, macaddress)
	if err != nil {
		return models.Node{}, err
	}
	collection := mongoconn.Client.Database("netmaker").Collection("nodes")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// Create filter
	filter := bson.M{"macaddress": macaddress, "network": network}
	node.SetLastModified()
	fmt.Println("Uncordoning node " + node.Name)
	// prepare update model.
	update := bson.D{
		{"$set", bson.D{
			{"ispending", false},
		}},
	}
	err = collection.FindOneAndUpdate(ctx, filter, update).Decode(&node)
	defer cancel()
	if err != nil {
		return models.Node{}, err
	}
	return node, nil
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
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

func CreateEgressGateway(gateway models.EgressGatewayRequest) (models.Node, error) {
	node, err := functions.GetNodeByMacAddress(gateway.NetID, gateway.NodeID)
	if err != nil {
		return models.Node{}, err
	}
	err = ValidateEgressGateway(gateway)
	if err != nil {
		return models.Node{}, err
	}
	var nodechange models.Node
	nodechange.IsEgressGateway = true
	nodechange.EgressGatewayRange = gateway.RangeString
	if gateway.PostUp == "" {
		nodechange.PostUp = "iptables -A FORWARD -i " + node.Interface + " -j ACCEPT; iptables -t nat -A POSTROUTING -o " + gateway.Interface + " -j MASQUERADE"
	} else {
		nodechange.PostUp = gateway.PostUp
	}
	if gateway.PostDown == "" {
		nodechange.PostDown = "iptables -D FORWARD -i " + node.Interface + " -j ACCEPT; iptables -t nat -D POSTROUTING -o " + gateway.Interface + " -j MASQUERADE"
	} else {
		nodechange.PostDown = gateway.PostDown
	}

	collection := mongoconn.Client.Database("netmaker").Collection("nodes")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// Create filter
	filter := bson.M{"macaddress": gateway.NodeID, "network": gateway.NetID}
	nodechange.SetLastModified()
	// prepare update model.
	update := bson.D{
		{"$set", bson.D{
			{"postup", nodechange.PostUp},
			{"postdown", nodechange.PostDown},
			{"isgateway", nodechange.IsEgressGateway},
			{"gatewayrange", nodechange.EgressGatewayRange},
			{"lastmodified", nodechange.LastModified},
		}},
	}
	var nodeupdate models.Node
	err = collection.FindOneAndUpdate(ctx, filter, update).Decode(&nodeupdate)
	defer cancel()
	if err != nil {
		return models.Node{}, err
	}
	err = SetNetworkNodesLastModified(gateway.NetID)
	if err != nil {
		return models.Node{}, err
	}
	//Get updated values to return
	node, err = functions.GetNodeByMacAddress(gateway.NetID, gateway.NodeID)
	if err != nil {
		return models.Node{}, err
	}
	return node, nil
}

func ValidateEgressGateway(gateway models.EgressGatewayRequest) error {
	var err error
	isIp := functions.IsIpCIDR(gateway.RangeString)
	empty := gateway.RangeString == ""
	if empty || !isIp {
		err = errors.New("IP Range Not Valid")
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
	node, err := DeleteEgressGateway(params["network"], params["macaddress"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

func DeleteEgressGateway(network, macaddress string) (models.Node, error) {

	var nodeupdate models.Node
	var nodechange models.Node
	node, err := functions.GetNodeByMacAddress(network, macaddress)
	if err != nil {
		return models.Node{}, err
	}

	nodechange.IsEgressGateway = false
	nodechange.EgressGatewayRange = ""
	nodechange.PostUp = ""
	nodechange.PostDown = ""

	collection := mongoconn.Client.Database("netmaker").Collection("nodes")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// Create filter
	filter := bson.M{"macaddress": macaddress, "network": network}
	nodechange.SetLastModified()
	// prepare update model.
	update := bson.D{
		{"$set", bson.D{
			{"postup", nodechange.PostUp},
			{"postdown", nodechange.PostDown},
			{"isgateway", nodechange.IsEgressGateway},
			{"gatewayrange", nodechange.EgressGatewayRange},
			{"lastmodified", nodechange.LastModified},
		}},
	}
	err = collection.FindOneAndUpdate(ctx, filter, update).Decode(&nodeupdate)
	defer cancel()
	if err != nil {
		return models.Node{}, err
	}
	err = SetNetworkNodesLastModified(network)
	if err != nil {
		return models.Node{}, err
	}
	//Get updated values to return
	node, err = functions.GetNodeByMacAddress(network, macaddress)
	if err != nil {
		return models.Node{}, err
	}
	return node, nil
}
// == INGRESS ==
func createIngressGateway(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	node, err := CreateIngressGateway(params["network"], params["macaddress"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

func CreateIngressGateway(network string, macaddress string) (models.Node, error) {
	node, err := functions.GetNodeByMacAddress(network, macaddress)
	if err != nil {
		return models.Node{}, err
	}

	collection := mongoconn.Client.Database("netmaker").Collection("nodes")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// Create filter
	filter := bson.M{"macaddress": macaddress, "network": network}
	// prepare update model.
	update := bson.D{
		{"$set", bson.D{
			{"isingress", true},
			{"lastmodified", time.Now().Unix()},
		}},
	}
	var nodeupdate models.Node
	err = collection.FindOneAndUpdate(ctx, filter, update).Decode(&nodeupdate)
	defer cancel()
	if err != nil {
		return models.Node{}, err
	}
	//Get updated values to return
	node, err = functions.GetNodeByMacAddress(network, macaddress)
	if err != nil {
		return models.Node{}, err
	}
	return node, nil
}

func deleteIngressGateway(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	node, err := DeleteIngressGateway(params["network"], params["macaddress"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

func DeleteIngressGateway(network, macaddress string) (models.Node, error) {

	var nodeupdate models.Node
	node, err := functions.GetNodeByMacAddress(network, macaddress)
	if err != nil {
		return models.Node{}, err
	}

	collection := mongoconn.Client.Database("netmaker").Collection("nodes")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// Create filter
	filter := bson.M{"macaddress": macaddress, "network": network}
	// prepare update model.
	update := bson.D{
		{"$set", bson.D{
			{"lastmodified", time.Now().Unix()},
			{"isingress", false},
		}},
	}
	err = collection.FindOneAndUpdate(ctx, filter, update).Decode(&nodeupdate)
	defer cancel()
	if err != nil {
		return models.Node{}, err
	}
	err = SetNetworkNodesLastModified(network)
	if err != nil {
		return models.Node{}, err
	}
	//Get updated values to return
	node, err = functions.GetNodeByMacAddress(network, macaddress)
	if err != nil {
		return models.Node{}, err
	}
	return node, nil
}

func updateNode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	//Get id from parameters
	//id, _ := primitive.ObjectIDFromHex(params["id"])

	var node models.Node

	//start here
	node, err := functions.GetNodeByMacAddress(params["network"], params["macaddress"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	var nodechange models.NodeUpdate

	// we decode our body request params
	_ = json.NewDecoder(r.Body).Decode(&nodechange)
	if nodechange.Network == "" {
		nodechange.Network = node.Network
	}
	if nodechange.MacAddress == "" {
		nodechange.MacAddress = node.MacAddress
	}
	err = ValidateNodeUpdate(params["network"], nodechange)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}

	node, err = UpdateNode(nodechange, node)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

//Delete a node
//Pretty straightforward
func deleteNode(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params
	var params = mux.Vars(r)

	success, err := DeleteNode(params["macaddress"], params["network"])

	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	} else if !success {
		err = errors.New("Could not delete node " + params["macaddress"])
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	returnSuccessResponse(w, r, params["macaddress"]+" deleted.")
}
