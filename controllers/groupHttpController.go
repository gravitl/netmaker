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

func groupHandlers(r *mux.Router) {
    r.HandleFunc("/api/groups", securityCheck(http.HandlerFunc(getGroups))).Methods("GET")
    r.HandleFunc("/api/groups", securityCheck(http.HandlerFunc(createGroup))).Methods("POST")
    r.HandleFunc("/api/groups/{groupname}", securityCheck(http.HandlerFunc(getGroup))).Methods("GET")
    r.HandleFunc("/api/groups/{groupname}", securityCheck(http.HandlerFunc(updateGroup))).Methods("PUT")
    r.HandleFunc("/api/groups/{groupname}", securityCheck(http.HandlerFunc(deleteGroup))).Methods("DELETE")
    r.HandleFunc("/api/groups/{groupname}/keyupdate", securityCheck(http.HandlerFunc(keyUpdate))).Methods("POST")
    r.HandleFunc("/api/groups/{groupname}/keys", securityCheck(http.HandlerFunc(createAccessKey))).Methods("POST")
    r.HandleFunc("/api/groups/{groupname}/keys", securityCheck(http.HandlerFunc(getAccessKeys))).Methods("GET")
    r.HandleFunc("/api/groups/{groupname}/keys/{name}", securityCheck(http.HandlerFunc(deleteAccessKey))).Methods("DELETE")
}

//Security check is middleware for every function and just checks to make sure that its the master calling
//Only admin should have access to all these group-level actions
//or maybe some Users once implemented
func securityCheck(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
		}

		var params = mux.Vars(r)
		hasgroup := params["groupname"] != ""
		groupexists, err := functions.GroupExists(params["groupname"])
                if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		} else if hasgroup && !groupexists {
                        errorResponse = models.ErrorResponse{
                                Code: http.StatusNotFound, Message: "W1R3: This group does not exist.",
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

//simple get all groups function
func getGroups(w http.ResponseWriter, r *http.Request) {

	groups, err := functions.ListGroups()

	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	} else {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(groups)
		return
	}
}

func validateGroup(operation string, group models.Group) error {

        v := validator.New()

        _ = v.RegisterValidation("addressrange_valid", func(fl validator.FieldLevel) bool {
		isvalid := functions.IsIpv4CIDR(fl.Field().String())
                return isvalid
        })

        _ = v.RegisterValidation("privaterange_valid", func(fl validator.FieldLevel) bool {
                isvalid := !*group.IsPrivate || functions.IsIpv4CIDR(fl.Field().String())
                return isvalid
        })

        _ = v.RegisterValidation("nameid_valid", func(fl validator.FieldLevel) bool {
		isFieldUnique := false
		inCharSet := false
		if operation == "update" { isFieldUnique = true } else{
			isFieldUnique, _ = functions.IsGroupNameUnique(fl.Field().String())
			inCharSet        = functions.NameInGroupCharSet(fl.Field().String())
		}
		return isFieldUnique && inCharSet
        })

        _ = v.RegisterValidation("displayname_unique", func(fl validator.FieldLevel) bool {
                isFieldUnique, _ := functions.IsGroupDisplayNameUnique(fl.Field().String())
                return isFieldUnique ||  operation == "update"
        })

        err := v.Struct(group)

        if err != nil {
                for _, e := range err.(validator.ValidationErrors) {
                        fmt.Println(e)
                }
        }
        return err
}

//Simple get group function
func getGroup(w http.ResponseWriter, r *http.Request) {

        // set header.
        w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

        var group models.Group

        collection := mongoconn.Client.Database("netmaker").Collection("groups")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"nameid": params["groupname"]}
        err := collection.FindOne(ctx, filter, options.FindOne().SetProjection(bson.M{"_id": 0})).Decode(&group)

        defer cancel()

        if err != nil {
		returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }
	w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(group)
}

func keyUpdate(w http.ResponseWriter, r *http.Request) {

        w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

        var group models.Group

        group, err := functions.GetParentGroup(params["groupname"])
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
	}


	group.KeyUpdateTimeStamp = time.Now().Unix()

        collection := mongoconn.Client.Database("netmaker").Collection("groups")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"nameid": params["groupname"]}

        // prepare update model.
        update := bson.D{
                {"$set", bson.D{
                        {"addressrange", group.AddressRange},
                        {"displayname", group.DisplayName},
                        {"defaultlistenport", group.DefaultListenPort},
                        {"defaultpostup", group.DefaultPostUp},
                        {"defaultpreup", group.DefaultPreUp},
			{"defaultkeepalive", group.DefaultKeepalive},
                        {"keyupdatetimestamp", group.KeyUpdateTimeStamp},
                        {"defaultsaveconfig", group.DefaultSaveConfig},
                        {"defaultinterface", group.DefaultInterface},
                        {"nodeslastmodified", group.NodesLastModified},
                        {"grouplastmodified", group.GroupLastModified},
                        {"allowmanualsignup", group.AllowManualSignUp},
                        {"defaultcheckininterval", group.DefaultCheckInInterval},
                }},
        }

        err = collection.FindOneAndUpdate(ctx, filter, update).Decode(&group)

        defer cancel()

        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(group)
}

//Update a group
func updateGroup(w http.ResponseWriter, r *http.Request) {

        w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

        var group models.Group

	group, err := functions.GetParentGroup(params["groupname"])
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

        var groupChange models.Group

	haschange := false
	hasrangeupdate := false
	hasprivaterangeupdate := false

	_ = json.NewDecoder(r.Body).Decode(&groupChange)

	if groupChange.AddressRange == "" {
		groupChange.AddressRange = group.AddressRange
	}
	if groupChange.NameID == "" {
		groupChange.NameID =  group.NameID
	}


        err = validateGroup("update", groupChange)
        if err != nil {
		returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

	//NOTE: Group.NameID is intentionally NOT editable. It acts as a static ID for the group. 
	//DisplayName can be changed instead, which is what shows on the front end

        if groupChange.AddressRange != "" {

            group.AddressRange = groupChange.AddressRange

	    var isAddressOK bool = functions.IsIpv4CIDR(groupChange.AddressRange)
            if !isAddressOK {
		    err := errors.New("Invalid Range of " +  groupChange.AddressRange + " for addresses.")
		    returnErrorResponse(w,r,formatError(err, "internal"))
                    return
            }
             haschange = true
	     hasrangeupdate = true

        }
	if groupChange.PrivateRange != "" {
            group.PrivateRange = groupChange.PrivateRange

            var isAddressOK bool = functions.IsIpv4CIDR(groupChange.PrivateRange)
            if !isAddressOK {
		    err := errors.New("Invalid Range of " +  groupChange.PrivateRange + " for internal addresses.")
                    returnErrorResponse(w,r,formatError(err, "internal"))
                    return
            }
             haschange = true
             hasprivaterangeupdate = true
	}
	if groupChange.IsPrivate != nil {
		group.IsPrivate = groupChange.IsPrivate
	}
	if groupChange.DefaultListenPort != 0 {
		group.DefaultListenPort = groupChange.DefaultListenPort
		haschange = true
        }
        if groupChange.DefaultPreUp != "" {
		group.DefaultPreUp = groupChange.DefaultPreUp
		haschange = true
        }
        if groupChange.DefaultInterface != "" {
		group.DefaultInterface = groupChange.DefaultInterface
		haschange = true
        }
        if groupChange.DefaultPostUp != "" {
		group.DefaultPostUp = groupChange.DefaultPostUp
		haschange = true
        }
        if groupChange.DefaultKeepalive != 0 {
		group.DefaultKeepalive = groupChange.DefaultKeepalive
		haschange = true
        }
        if groupChange.DisplayName != "" {
		group.DisplayName = groupChange.DisplayName
		haschange = true
        }
        if groupChange.DefaultCheckInInterval != 0 {
		group.DefaultCheckInInterval = groupChange.DefaultCheckInInterval
		haschange = true
        }
        if groupChange.AllowManualSignUp != nil {
		group.AllowManualSignUp = groupChange.AllowManualSignUp
		haschange = true
        }

        collection := mongoconn.Client.Database("netmaker").Collection("groups")
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        filter := bson.M{"nameid": params["groupname"]}

	if haschange {
		group.SetGroupLastModified()
	}

        // prepare update model.
        update := bson.D{
                {"$set", bson.D{
                        {"addressrange", group.AddressRange},
                        {"displayname", group.DisplayName},
                        {"defaultlistenport", group.DefaultListenPort},
                        {"defaultpostup", group.DefaultPostUp},
                        {"defaultpreup", group.DefaultPreUp},
                        {"defaultkeepalive", group.DefaultKeepalive},
                        {"defaultsaveconfig", group.DefaultSaveConfig},
                        {"defaultinterface", group.DefaultInterface},
                        {"nodeslastmodified", group.NodesLastModified},
                        {"grouplastmodified", group.GroupLastModified},
                        {"allowmanualsignup", group.AllowManualSignUp},
                        {"privaterange", group.PrivateRange},
                        {"isprivate", group.IsPrivate},
                        {"defaultcheckininterval", group.DefaultCheckInInterval},
		}},
        }

	err = collection.FindOneAndUpdate(ctx, filter, update).Decode(&group)
        defer cancel()

        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

	//Cycles through nodes and gives them new IP's based on the new range
	//Pretty cool, but also pretty inefficient currently
        if hasrangeupdate {
		err = functions.UpdateGroupNodeAddresses(params["groupname"])
		if err != nil {
			returnErrorResponse(w,r,formatError(err, "internal"))
			return
		}
	}
	if hasprivaterangeupdate {
                err = functions.UpdateGroupPrivateAddresses(params["groupname"])
                if err != nil {
                        returnErrorResponse(w,r,formatError(err, "internal"))
                        return
                }
	}
	returngroup, err := functions.GetParentGroup(group.NameID)
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(returngroup)
}

//Delete a group
//Will stop you if  there's any nodes associated
func deleteGroup(w http.ResponseWriter, r *http.Request) {
        // Set header
        w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

	nodecount, err := functions.GetGroupNodeNumber(params["groupname"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	} else if nodecount > 0  {
		errorResponse := models.ErrorResponse{
                        Code: http.StatusForbidden, Message: "W1R3: Node check failed. All nodes must be deleted before deleting group.",
                }
                returnErrorResponse(w, r, errorResponse)
                return
        }

        collection := mongoconn.Client.Database("netmaker").Collection("groups")

        filter := bson.M{"nameid": params["groupname"]}

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

//Create a group
//Pretty simple
func createGroup(w http.ResponseWriter, r *http.Request) {

        w.Header().Set("Content-Type", "application/json")

        var group models.Group

        // we decode our body request params
	err := json.NewDecoder(r.Body).Decode(&group)
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

	//TODO: Not really doing good validation here. Same as createNode, updateNode, and updateGroup
	//Need to implement some better validation across the board
        if group.IsPrivate == nil {
                falsevar := false
                group.IsPrivate = &falsevar
        }

        err = validateGroup("create", group)
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }
	group.SetDefaults()
        group.SetNodesLastModified()
        group.SetGroupLastModified()
        group.KeyUpdateTimeStamp = time.Now().Unix()

        collection := mongoconn.Client.Database("netmaker").Collection("groups")
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)


        // insert our group into the group table
        result, err := collection.InsertOne(ctx, group)

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
//accesskey is created as a json string inside the Group collection item in mongo
func createAccessKey(w http.ResponseWriter, r *http.Request) {

        w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

        var group models.Group
        var accesskey models.AccessKey

        //start here
	group, err := functions.GetParentGroup(params["groupname"])
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

	network := params["groupname"]
	address := gconf.ServerGRPC + gconf.PortGRPC

	accessstringdec := address + "." + network + "." + accesskey.Value
	accesskey.AccessString = base64.StdEncoding.EncodeToString([]byte(accessstringdec))

	group.AccessKeys = append(group.AccessKeys, accesskey)

        collection := mongoconn.Client.Database("netmaker").Collection("groups")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        // Create filter
        filter := bson.M{"nameid": params["groupname"]}

        // Read update model from body request
        fmt.Println("Adding key to " + group.NameID)

        // prepare update model.
        update := bson.D{
                {"$set", bson.D{
                        {"accesskeys", group.AccessKeys},
                }},
        }

        err = collection.FindOneAndUpdate(ctx, filter, update).Decode(&group)

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

        var group models.Group
        //var keys []models.DisplayKey
	var keys []models.AccessKey
        collection := mongoconn.Client.Database("netmaker").Collection("groups")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"nameid": params["groupname"]}
        err := collection.FindOne(ctx, filter, options.FindOne().SetProjection(bson.M{"_id": 0})).Decode(&group)

        defer cancel()

        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }
	keydata, err := json.Marshal(group.AccessKeys)

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

        var group models.Group
	keyname := params["name"]

        //start here
	group, err := functions.GetParentGroup(params["groupname"])
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }
	//basically, turn the list of access keys into the list of access keys before and after the item
	//have not done any error handling for if there's like...1 item. I think it works? need to test.
	for i := len(group.AccessKeys) - 1; i >= 0; i-- {

		currentkey:= group.AccessKeys[i]
		if currentkey.Name == keyname {
			group.AccessKeys = append(group.AccessKeys[:i],
				group.AccessKeys[i+1:]...)
		}
	}

        collection := mongoconn.Client.Database("netmaker").Collection("groups")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        // Create filter
        filter := bson.M{"nameid": params["groupname"]}

        // prepare update model.
        update := bson.D{
                {"$set", bson.D{
                        {"accesskeys", group.AccessKeys},
                }},
        }

        err = collection.FindOneAndUpdate(ctx, filter, update).Decode(&group)

        defer cancel()

        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }
        var keys []models.AccessKey
	keydata, err := json.Marshal(group.AccessKeys)
        if err != nil {
                returnErrorResponse(w,r,formatError(err, "internal"))
                return
        }

        json.Unmarshal(keydata, &keys)

        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(keys)
}
