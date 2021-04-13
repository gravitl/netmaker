package controller

import (
    "github.com/gravitl/netmaker/models"
    "errors"
    "github.com/gravitl/netmaker/functions"
    "github.com/gravitl/netmaker/mongoconn"
    "golang.org/x/crypto/bcrypt"
    "time"
    "strings"
    "fmt"
    "context"
    "encoding/json"
    "net/http"
    "github.com/gorilla/mux"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo/options"
)


func nodeHandlers(r *mux.Router) {

    r.HandleFunc("/api/nodes", authorize(false, "master", http.HandlerFunc(getAllNodes))).Methods("GET")
    r.HandleFunc("/api/nodes/{group}", authorize(true, "group", http.HandlerFunc(getGroupNodes))).Methods("GET")
    r.HandleFunc("/api/nodes/{group}/{macaddress}", authorize(true, "node", http.HandlerFunc(getNode))).Methods("GET")
    r.HandleFunc("/api/nodes/{group}/{macaddress}", authorize(true, "node", http.HandlerFunc(updateNode))).Methods("PUT")
    r.HandleFunc("/api/nodes/{group}/{macaddress}", authorize(true, "node", http.HandlerFunc(deleteNode))).Methods("DELETE")
    r.HandleFunc("/api/nodes/{group}/{macaddress}/checkin", authorize(true, "node", http.HandlerFunc(checkIn))).Methods("POST")
//    r.HandleFunc("/api/nodes/{group}/{macaddress}/creategateway", authorize(true, "master", http.HandlerFunc(createGateway))).Methods("POST")
//    r.HandleFunc("/api/nodes/{group}/{macaddress}/deletegateway", authorize(true, "master", http.HandlerFunc(deleteGateway))).Methods("POST")
    r.HandleFunc("/api/nodes/{group}/{macaddress}/uncordon", authorize(true, "master", http.HandlerFunc(uncordonNode))).Methods("POST")
    r.HandleFunc("/api/nodes/{group}/nodes", createNode).Methods("POST")
    r.HandleFunc("/api/nodes/adm/{group}/lastmodified", authorize(true, "group", http.HandlerFunc(getLastModified))).Methods("GET")
    r.HandleFunc("/api/nodes/adm/{group}/authenticate", authenticate).Methods("POST")

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
	    var err = collection.FindOne(ctx, bson.M{ "macaddress": authRequest.MacAddress, "ispending": false }).Decode(&result)

            defer cancel()

            if err != nil {
                returnErrorResponse(response, request, errorResponse)
		return
            }

	   //compare password from request to stored password in database
	   //might be able to have a common hash (certificates?) and compare those so that a password isn't passed in in plain text...
	   //TODO: Consider a way of hashing the password client side before sending, or using certificates
	   err = bcrypt.CompareHashAndPassword([]byte(result.Password), []byte(authRequest.Password))
	   if err != nil {
		   returnErrorResponse(response, request, errorResponse)
		   return
	   } else {
		//Create a new JWT for the node
                tokenString, _ := functions.CreateJWT(authRequest.MacAddress, result.Group)

                if tokenString == "" {
                    returnErrorResponse(response, request, errorResponse)
		    return
                }

                var successResponse = models.SuccessResponse{
                    Code:    http.StatusOK,
                    Message: "W1R3: Device " + authRequest.MacAddress + " Authorized",
                    Response: models.SuccessfulLoginResponse{
                        AuthToken: tokenString,
                        MacAddress:     authRequest.MacAddress,
                    },
                }
                //Send back the JWT
                successJSONResponse, jsonError := json.Marshal(successResponse)

                if jsonError != nil {
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
//This will also check against the authGroup and make sure the node should be accessing that endpoint,
//even if it's technically ok
//This is kind of a poor man's RBAC. There's probably a better/smarter way.
//TODO: Consider better RBAC implementations
func authorize(groupCheck bool, authGroup string, next http.Handler) http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {

                var errorResponse = models.ErrorResponse{
                        Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
                }

		var params = mux.Vars(r)

		groupexists, _ := functions.GroupExists(params["group"])

		//check that the request is for a valid group
                //if (groupCheck && !groupexists) || err != nil {
                if (groupCheck && !groupexists) {
                        errorResponse = models.ErrorResponse{
                                Code: http.StatusNotFound, Message: "W1R3: This group does not exist. ",
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
                }  else {
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
		//So each route defines which access group should be allowed to access it
		} else {
			switch authGroup {
			case "all":
				isAuthorized = true
                        case "nodes":
				isAuthorized = (macaddress != "")
                        case "group":
                                node, err := functions.GetNodeByMacAddress(params["group"], macaddress)
		                if err != nil {
					errorResponse = models.ErrorResponse{
					Code: http.StatusUnauthorized, Message: "W1R3: Missing Auth Token.",
					}
					returnErrorResponse(w, r, errorResponse)
					return
		                }
                                isAuthorized = (node.Group == params["group"])
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

//Gets all nodes associated with group, including pending nodes
func getGroupNodes(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var nodes []models.ReturnNode
	var params = mux.Vars(r)

	collection := mongoconn.Client.Database("netmaker").Collection("nodes")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"group": params["group"]}

	//Filtering out the ID field cuz Dillon doesn't like it. May want to filter out other fields in the future
	cur, err := collection.Find(ctx, filter, options.Find().SetProjection(bson.M{"_id": 0}))

        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

	defer cancel()

	for cur.Next(context.TODO()) {

		//Using a different model for the ReturnNode (other than regular node).
		//Either we should do this for ALL structs (so Groups and Keys)
		//OR we should just use the original struct
		//My preference is to make some new return structs
		//TODO: Think about this. Not an immediate concern. Just need to get some consistency eventually
		var node models.ReturnNode

		err := cur.Decode(&node)
	        if err != nil {
			returnErrorResponse(w,r,formatError(err, "internal"))
			return
		}

		// add item our array of nodes
		nodes = append(nodes, node)
	}

	//TODO: Another fatal error we should take care of.
	if err := cur.Err(); err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
	}

	//Returns all the nodes in JSON format
        w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(nodes)

}

//A separate function to get all nodes, not just nodes for a particular group.
//Not quite sure if this is necessary. Probably necessary based on front end but may want to review after iteration 1 if it's being used or not
func getAllNodes(w http.ResponseWriter, r *http.Request) {

        w.Header().Set("Content-Type", "application/json")

        var nodes []models.ReturnNode

        collection := mongoconn.Client.Database("netmaker").Collection("nodes")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	// Filter out them ID's again
	cur, err := collection.Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"_id": 0}))
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

        defer cancel()

        for cur.Next(context.TODO()) {

                var node models.ReturnNode
                err := cur.Decode(&node)
	        if err != nil {
		        returnErrorResponse(w,r,formatError(err, "internal"))
			return
		}
                // add node to our array
                nodes = append(nodes, node)
        }

	//TODO: Fatal error
        if err := cur.Err(); err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

	//Return all the nodes in JSON format
        w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(nodes)

}

//This function get's called when a node "checks in" at check in interval
//Honestly I'm not sure what all it should be doing
//TODO: Implement the necessary stuff, including the below
//Check the last modified of the group
//Check the last modified of the nodes
//Write functions for responding to these two thingies
func checkIn(w http.ResponseWriter, r *http.Request) {

	//TODO: Current thoughts:
	//Dont bother with a grouplastmodified
	//Instead, implement a "configupdate" boolean on nodes
	//when there is a group update  that requrires  a config update,  then the node will pull its new config

        // set header.
        w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

	var node models.Node


	//Retrieves node with DB Call which is inefficient. Let's just get the time and set it.
	//node = functions.GetNodeByMacAddress(params["group"], params["macaddress"])

        collection := mongoconn.Client.Database("netmaker").Collection("nodes")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"macaddress": params["macaddress"], "group": params["group"]}

	//old code was inefficient, this is all we need.
	time := time.Now().String()

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
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

	//TODO: check node last modified vs group last modified
        w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)

}

//Get an individual node. Nothin fancy here folks.
func getNode(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

	node, err := GetNode(params["macaddress"], params["group"])
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

//Get the time that a group of nodes was last modified.
//TODO: This needs to be refactored
//Potential way to do this: On UpdateNode, set a new field for "LastModified"
//If we go with the existing way, we need to at least set group.NodesLastModified on UpdateNode
func getLastModified(w http.ResponseWriter, r *http.Request) {
        // set header.
        w.Header().Set("Content-Type", "application/json")

        var group models.Group
        var params = mux.Vars(r)

        collection := mongoconn.Client.Database("netmaker").Collection("groups")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"nameid": params["group"]}
        err := collection.FindOne(ctx, filter).Decode(&group)

	defer cancel()

        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(string(group.NodesLastModified)))

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

        groupName := params["group"]

	//Check if group exists  first
	//TODO: This is inefficient. Let's find a better way. 
	//Just a few rows down we grab the group anyway
        groupexists, err := functions.GroupExists(groupName)

        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        } else if !groupexists {
                errorResponse = models.ErrorResponse{
                        Code: http.StatusNotFound, Message: "W1R3: Group does not exist! ",
                }
                returnErrorResponse(w, r, errorResponse)
                return
        }

	var node models.Node

	//get node from body of request
	err = json.NewDecoder(r.Body).Decode(&node)
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

	node.Group = groupName


	group, err := node.GetGroup()
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

	//Check to see if key is valid
	//TODO: Triple inefficient!!! This is the third call to the DB we make for groups
	validKey := functions.IsKeyValid(groupName, node.AccessKey)

	if !validKey {
		//Check to see if group will allow manual sign up
		//may want to switch this up with the valid key check and avoid a DB call that way.
		if *group.AllowManualSignUp {
			node.IsPending = true
		} else  {
			errorResponse = models.ErrorResponse{
				Code: http.StatusUnauthorized, Message: "W1R3: Key invalid, or none provided.",
			}
			returnErrorResponse(w, r, errorResponse)
			return
		}
	}

	err =  ValidateNode("create", groupName, node)
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

        node, err = CreateNode(node, groupName)
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(node)
}

//Takes node out of pending state
//TODO: May want to use cordon/uncordon terminology instead of "ispending".
func uncordonNode(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

        var node models.Node

	node, err := functions.GetNodeByMacAddress(params["group"], params["macaddress"])
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

        collection := mongoconn.Client.Database("netmaker").Collection("nodes")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        // Create filter
	filter := bson.M{"macaddress": params["macaddress"], "group": params["group"]}

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
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

        fmt.Println("Node " + node.Name + " uncordoned.")
	w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode("SUCCESS")
}

func createGateway(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

        var node models.Node

        node, err := functions.GetNodeByMacAddress(params["group"], params["macaddress"])
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

        collection := mongoconn.Client.Database("netmaker").Collection("nodes")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        // Create filter
        filter := bson.M{"macaddress": params["macaddress"], "group": params["group"]}

        node.SetLastModified()

        err =  ValidateNode("create", params["group"], node)
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

        // prepare update model.
        update := bson.D{
                {"$set", bson.D{
                        {"ispending", false},
                }},
        }

        err = collection.FindOneAndUpdate(ctx, filter, update).Decode(&node)

        defer cancel()

        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }
        fmt.Println("Node " + node.Name + " uncordoned.")

	w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode("SUCCESS")
}



func updateNode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	//Get id from parameters
	//id, _ := primitive.ObjectIDFromHex(params["id"])

	var node models.Node

	//start here
	node, err := functions.GetNodeByMacAddress(params["group"], params["macaddress"])
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }



        var nodechange models.Node

        // we decode our body request params
        _ = json.NewDecoder(r.Body).Decode(&nodechange)
	if nodechange.Group == "" {
	        nodechange.Group = node.Group
	}
	if nodechange.MacAddress == "" {
		nodechange.MacAddress = node.MacAddress
	}

        err = ValidateNode("update", params["group"], nodechange)
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

	node, err = UpdateNode(nodechange, node)
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
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

	success, err := DeleteNode(params["macaddress"], params["group"])

	if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        } else if !success {
		err = errors.New("Could not delete node " + params["macaddress"])
                returnErrorResponse(w,r,formatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(params["macaddress"] + " deleted.")
}
