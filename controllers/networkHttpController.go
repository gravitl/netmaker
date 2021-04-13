package controller

import (
    "gopkg.in/go-playground/validator.v9"
    "github.com/gravitl/netmaker/models"
    "errors"
    "encoding/base64"
    "github.com/gravitl/netmaker/functions"
    "github.com/gravitl/netmaker/mongoconn"
    "time"
    "strings"
    "fmt"
    "context"
    "encoding/json"
    "net/http"
    "github.com/gorilla/mux"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo/options"
    "github.com/gravitl/netmaker/config"
)

func networkHandlers(r *mux.Router) {
    r.HandleFunc("/api/networks", securityCheck(http.HandlerFunc(getNetworks))).Methods("GET")
    r.HandleFunc("/api/networks", securityCheck(http.HandlerFunc(createNetwork))).Methods("POST")
    r.HandleFunc("/api/networks/{networkname}", securityCheck(http.HandlerFunc(getNetwork))).Methods("GET")
    r.HandleFunc("/api/networks/{networkname}", securityCheck(http.HandlerFunc(updateNetwork))).Methods("PUT")
    r.HandleFunc("/api/networks/{networkname}", securityCheck(http.HandlerFunc(deleteNetwork))).Methods("DELETE")
    r.HandleFunc("/api/networks/{networkname}/keyupdate", securityCheck(http.HandlerFunc(keyUpdate))).Methods("POST")
    r.HandleFunc("/api/networks/{networkname}/keys", securityCheck(http.HandlerFunc(createAccessKey))).Methods("POST")
    r.HandleFunc("/api/networks/{networkname}/keys", securityCheck(http.HandlerFunc(getAccessKeys))).Methods("GET")
    r.HandleFunc("/api/networks/{networkname}/keys/{name}", securityCheck(http.HandlerFunc(deleteAccessKey))).Methods("DELETE")
}

//Security check is middleware for every function and just checks to make sure that its the master calling
//Only admin should have access to all these network-level actions
//or maybe some Users once implemented
func securityCheck(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
		}

		var params = mux.Vars(r)
		hasnetwork := params["networkname"] != ""
		networkexists, err := functions.NetworkExists(params["networkname"])
                if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		} else if hasnetwork && !networkexists {
                        errorResponse = models.ErrorResponse{
                                Code: http.StatusNotFound, Message: "W1R3: This network does not exist.",
                        }
                        returnErrorResponse(w, r, errorResponse)
			return
                } else {

		bearerToken := r.Header.Get("Authorization")

		var hasBearer = true
		var tokenSplit = strings.Split(bearerToken, " ")
		var  authToken = ""

		if len(tokenSplit) < 2 {
			hasBearer = false
		} else {
			authToken = tokenSplit[1]
		}
		//all endpoints here require master so not as complicated
		//still might not be a good  way of doing this
		if !hasBearer || !authenticateMaster(authToken) {
			errorResponse = models.ErrorResponse{
				Code: http.StatusUnauthorized, Message: "W1R3: You are unauthorized to access this endpoint.",
			}
			returnErrorResponse(w, r, errorResponse)
			return
		} else {
			next.ServeHTTP(w, r)
		}
		}
	}
}
//Consider a more secure way of setting master key
func authenticateMaster(tokenString string) bool {
    if tokenString == config.Config.Server.MasterKey {
        return true
    }
    return false
}

//simple get all networks function
func getNetworks(w http.ResponseWriter, r *http.Request) {

	networks, err := functions.ListNetworks()

	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	} else {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(networks)
		return
	}
}

func validateNetwork(operation string, network models.Network) error {

        v := validator.New()

        _ = v.RegisterValidation("addressrange_valid", func(fl validator.FieldLevel) bool {
		isvalid := functions.IsIpv4CIDR(fl.Field().String())
                return isvalid
        })

        _ = v.RegisterValidation("privaterange_valid", func(fl validator.FieldLevel) bool {
                isvalid := !*network.IsPrivate || functions.IsIpv4CIDR(fl.Field().String())
                return isvalid
        })

        _ = v.RegisterValidation("netid_valid", func(fl validator.FieldLevel) bool {
		isFieldUnique := false
		inCharSet := false
		if operation == "update" { isFieldUnique = true } else{
			isFieldUnique, _ = functions.IsNetworkNameUnique(fl.Field().String())
			inCharSet        = functions.NameInNetworkCharSet(fl.Field().String())
		}
		return isFieldUnique && inCharSet
        })

        _ = v.RegisterValidation("displayname_unique", func(fl validator.FieldLevel) bool {
                isFieldUnique, _ := functions.IsNetworkDisplayNameUnique(fl.Field().String())
                return isFieldUnique ||  operation == "update"
        })

        err := v.Struct(network)

        if err != nil {
                for _, e := range err.(validator.ValidationErrors) {
                        fmt.Println(e)
                }
        }
        return err
}

//Simple get network function
func getNetwork(w http.ResponseWriter, r *http.Request) {

        // set header.
        w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

        var network models.Network

        collection := mongoconn.Client.Database("netmaker").Collection("networks")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"netid": params["networkname"]}
        err := collection.FindOne(ctx, filter, options.FindOne().SetProjection(bson.M{"_id": 0})).Decode(&network)

        defer cancel()

        if err != nil {
		returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }
	w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(network)
}

func keyUpdate(w http.ResponseWriter, r *http.Request) {

        w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

        var network models.Network

        network, err := functions.GetParentNetwork(params["networkname"])
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
	}


	network.KeyUpdateTimeStamp = time.Now().Unix()

        collection := mongoconn.Client.Database("netmaker").Collection("networks")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"netid": params["networkname"]}

        // prepare update model.
        update := bson.D{
                {"$set", bson.D{
                        {"addressrange", network.AddressRange},
                        {"displayname", network.DisplayName},
                        {"defaultlistenport", network.DefaultListenPort},
                        {"defaultpostup", network.DefaultPostUp},
                        {"defaultpreup", network.DefaultPreUp},
			{"defaultkeepalive", network.DefaultKeepalive},
                        {"keyupdatetimestamp", network.KeyUpdateTimeStamp},
                        {"defaultsaveconfig", network.DefaultSaveConfig},
                        {"defaultinterface", network.DefaultInterface},
                        {"nodeslastmodified", network.NodesLastModified},
                        {"networklastmodified", network.NetworkLastModified},
                        {"allowmanualsignup", network.AllowManualSignUp},
                        {"defaultcheckininterval", network.DefaultCheckInInterval},
                }},
        }

        err = collection.FindOneAndUpdate(ctx, filter, update).Decode(&network)

        defer cancel()

        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(network)
}

//Update a network
func updateNetwork(w http.ResponseWriter, r *http.Request) {

        w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

        var network models.Network

	network, err := functions.GetParentNetwork(params["networkname"])
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

        var networkChange models.Network

	haschange := false
	hasrangeupdate := false
	hasprivaterangeupdate := false

	_ = json.NewDecoder(r.Body).Decode(&networkChange)

	if networkChange.AddressRange == "" {
		networkChange.AddressRange = network.AddressRange
	}
	if networkChange.NetID == "" {
		networkChange.NetID =  network.NetID
	}


        err = validateNetwork("update", networkChange)
        if err != nil {
		returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

	//NOTE: Network.NetID is intentionally NOT editable. It acts as a static ID for the network. 
	//DisplayName can be changed instead, which is what shows on the front end

        if networkChange.AddressRange != "" {

            network.AddressRange = networkChange.AddressRange

	    var isAddressOK bool = functions.IsIpv4CIDR(networkChange.AddressRange)
            if !isAddressOK {
		    err := errors.New("Invalid Range of " +  networkChange.AddressRange + " for addresses.")
		    returnErrorResponse(w,r,formatError(err, "internal"))
                    return
            }
             haschange = true
	     hasrangeupdate = true

        }
	if networkChange.PrivateRange != "" {
            network.PrivateRange = networkChange.PrivateRange

            var isAddressOK bool = functions.IsIpv4CIDR(networkChange.PrivateRange)
            if !isAddressOK {
		    err := errors.New("Invalid Range of " +  networkChange.PrivateRange + " for internal addresses.")
                    returnErrorResponse(w,r,formatError(err, "internal"))
                    return
            }
             haschange = true
             hasprivaterangeupdate = true
	}
	if networkChange.IsPrivate != nil {
		network.IsPrivate = networkChange.IsPrivate
	}
	if networkChange.DefaultListenPort != 0 {
		network.DefaultListenPort = networkChange.DefaultListenPort
		haschange = true
        }
        if networkChange.DefaultPreUp != "" {
		network.DefaultPreUp = networkChange.DefaultPreUp
		haschange = true
        }
        if networkChange.DefaultInterface != "" {
		network.DefaultInterface = networkChange.DefaultInterface
		haschange = true
        }
        if networkChange.DefaultPostUp != "" {
		network.DefaultPostUp = networkChange.DefaultPostUp
		haschange = true
        }
        if networkChange.DefaultKeepalive != 0 {
		network.DefaultKeepalive = networkChange.DefaultKeepalive
		haschange = true
        }
        if networkChange.DisplayName != "" {
		network.DisplayName = networkChange.DisplayName
		haschange = true
        }
        if networkChange.DefaultCheckInInterval != 0 {
		network.DefaultCheckInInterval = networkChange.DefaultCheckInInterval
		haschange = true
        }
        if networkChange.AllowManualSignUp != nil {
		network.AllowManualSignUp = networkChange.AllowManualSignUp
		haschange = true
        }

        collection := mongoconn.Client.Database("netmaker").Collection("networks")
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        filter := bson.M{"netid": params["networkname"]}

	if haschange {
		network.SetNetworkLastModified()
	}

        // prepare update model.
        update := bson.D{
                {"$set", bson.D{
                        {"addressrange", network.AddressRange},
                        {"displayname", network.DisplayName},
                        {"defaultlistenport", network.DefaultListenPort},
                        {"defaultpostup", network.DefaultPostUp},
                        {"defaultpreup", network.DefaultPreUp},
                        {"defaultkeepalive", network.DefaultKeepalive},
                        {"defaultsaveconfig", network.DefaultSaveConfig},
                        {"defaultinterface", network.DefaultInterface},
                        {"nodeslastmodified", network.NodesLastModified},
                        {"networklastmodified", network.NetworkLastModified},
                        {"allowmanualsignup", network.AllowManualSignUp},
                        {"privaterange", network.PrivateRange},
                        {"isprivate", network.IsPrivate},
                        {"defaultcheckininterval", network.DefaultCheckInInterval},
		}},
        }

	err = collection.FindOneAndUpdate(ctx, filter, update).Decode(&network)
        defer cancel()

        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

	//Cycles through nodes and gives them new IP's based on the new range
	//Pretty cool, but also pretty inefficient currently
        if hasrangeupdate {
		err = functions.UpdateNetworkNodeAddresses(params["networkname"])
		if err != nil {
			returnErrorResponse(w,r,formatError(err, "internal"))
			return
		}
	}
	if hasprivaterangeupdate {
                err = functions.UpdateNetworkPrivateAddresses(params["networkname"])
                if err != nil {
                        returnErrorResponse(w,r,formatError(err, "internal"))
                        return
                }
	}
	returnnetwork, err := functions.GetParentNetwork(network.NetID)
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(returnnetwork)
}

//Delete a network
//Will stop you if  there's any nodes associated
func deleteNetwork(w http.ResponseWriter, r *http.Request) {
        // Set header
        w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

	nodecount, err := functions.GetNetworkNodeNumber(params["networkname"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	} else if nodecount > 0  {
		errorResponse := models.ErrorResponse{
                        Code: http.StatusForbidden, Message: "W1R3: Node check failed. All nodes must be deleted before deleting network.",
                }
                returnErrorResponse(w, r, errorResponse)
                return
        }

        collection := mongoconn.Client.Database("netmaker").Collection("networks")

        filter := bson.M{"netid": params["networkname"]}

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        deleteResult, err := collection.DeleteOne(ctx, filter)

        defer cancel()

        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(deleteResult)
}

//Create a network
//Pretty simple
func createNetwork(w http.ResponseWriter, r *http.Request) {

        w.Header().Set("Content-Type", "application/json")

        var network models.Network

        // we decode our body request params
	err := json.NewDecoder(r.Body).Decode(&network)
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

	//TODO: Not really doing good validation here. Same as createNode, updateNode, and updateNetwork
	//Need to implement some better validation across the board
        if network.IsPrivate == nil {
                falsevar := false
                network.IsPrivate = &falsevar
        }

        err = validateNetwork("create", network)
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }
	network.SetDefaults()
        network.SetNodesLastModified()
        network.SetNetworkLastModified()
        network.KeyUpdateTimeStamp = time.Now().Unix()

        collection := mongoconn.Client.Database("netmaker").Collection("networks")
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)


        // insert our network into the network table
        result, err := collection.InsertOne(ctx, network)

        defer cancel()

        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(result)
}

// BEGIN KEY MANAGEMENT SECTION


//TODO: Very little error handling
//accesskey is created as a json string inside the Network collection item in mongo
func createAccessKey(w http.ResponseWriter, r *http.Request) {

        w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

        var network models.Network
        var accesskey models.AccessKey

        //start here
	network, err := functions.GetParentNetwork(params["networkname"])
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

        err = json.NewDecoder(r.Body).Decode(&accesskey)
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

	if accesskey.Name == "" {
                accesskey.Name = functions.GenKeyName()
        }
	if accesskey.Value == "" {
		accesskey.Value = functions.GenKey()
	}
        if accesskey.Uses == 0 {
                accesskey.Uses = 1
        }
	gconf, err := functions.GetGlobalConfig()
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

	privAddr := ""
	if *network.IsPrivate {
		privAddr = network.PrivateRange
	}


	netID := params["networkname"]
	address := gconf.ServerGRPC + gconf.PortGRPC

	accessstringdec := address + "." + netID + "." + accesskey.Value + "." + privAddr
	accesskey.AccessString = base64.StdEncoding.EncodeToString([]byte(accessstringdec))

	network.AccessKeys = append(network.AccessKeys, accesskey)

        collection := mongoconn.Client.Database("netmaker").Collection("networks")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        // Create filter
        filter := bson.M{"netid": params["networkname"]}

        // Read update model from body request
        fmt.Println("Adding key to " + network.NetID)

        // prepare update model.
        update := bson.D{
                {"$set", bson.D{
                        {"accesskeys", network.AccessKeys},
                }},
        }

        err = collection.FindOneAndUpdate(ctx, filter, update).Decode(&network)

        defer cancel()

	if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(accesskey)
	//w.Write([]byte(accesskey.AccessString))
}

//pretty simple get
func getAccessKeys(w http.ResponseWriter, r *http.Request) {

	// set header.
        w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

        var network models.Network
        //var keys []models.DisplayKey
	var keys []models.AccessKey
        collection := mongoconn.Client.Database("netmaker").Collection("networks")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"netid": params["networkname"]}
        err := collection.FindOne(ctx, filter, options.FindOne().SetProjection(bson.M{"_id": 0})).Decode(&network)

        defer cancel()

        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }
	keydata, err := json.Marshal(network.AccessKeys)

        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

	json.Unmarshal(keydata, &keys)

	w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(keys)
}


//delete key. Has to do a little funky logic since it's not a collection item
func deleteAccessKey(w http.ResponseWriter, r *http.Request) {

        w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

        var network models.Network
	keyname := params["name"]

        //start here
	network, err := functions.GetParentNetwork(params["networkname"])
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }
	//basically, turn the list of access keys into the list of access keys before and after the item
	//have not done any error handling for if there's like...1 item. I think it works? need to test.
	for i := len(network.AccessKeys) - 1; i >= 0; i-- {

		currentkey:= network.AccessKeys[i]
		if currentkey.Name == keyname {
			network.AccessKeys = append(network.AccessKeys[:i],
				network.AccessKeys[i+1:]...)
		}
	}

        collection := mongoconn.Client.Database("netmaker").Collection("networks")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        // Create filter
        filter := bson.M{"netid": params["networkname"]}

        // prepare update model.
        update := bson.D{
                {"$set", bson.D{
                        {"accesskeys", network.AccessKeys},
                }},
        }

        err = collection.FindOneAndUpdate(ctx, filter, update).Decode(&network)

        defer cancel()

        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }
        var keys []models.AccessKey
	keydata, err := json.Marshal(network.AccessKeys)
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

        json.Unmarshal(keydata, &keys)

        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(keys)
}
