package controller

import (
    "gopkg.in/go-playground/validator.v9"
    "github.com/gravitl/netmaker/models"
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
    r.HandleFunc("/api/groups/{groupname}/keyupdate", securityCheck(http.HandlerFunc(keyUpdate))).Methods("POST")
    r.HandleFunc("/api/groups/{groupname}", securityCheck(http.HandlerFunc(getGroup))).Methods("GET")
    r.HandleFunc("/api/groups/{groupname}/numnodes", securityCheck(http.HandlerFunc(getGroupNodeNumber))).Methods("GET")
    r.HandleFunc("/api/groups/{groupname}", securityCheck(http.HandlerFunc(updateGroup))).Methods("PUT")
    r.HandleFunc("/api/groups/{groupname}", securityCheck(http.HandlerFunc(deleteGroup))).Methods("DELETE")
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
		groupexists, _ := functions.GroupExists(params["groupname"])
                if hasgroup && !groupexists {
                        errorResponse = models.ErrorResponse{
                                Code: http.StatusNotFound, Message: "W1R3: This group does not exist.",
                        }
                        returnErrorResponse(w, r, errorResponse)
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

	//depends on list groups function
	//TODO: This is perhaps a more efficient way of handling ALL http handlers
	//Take their primary logic and put in a separate function
	//May be better since most http handler functionality is needed internally cross-method
	//E.G. a method may need to check against all groups. But it  cant call this function. That's why there's ListGroups
	groups := functions.ListGroups()

	json.NewEncoder(w).Encode(groups)

}

func validateGroup(operation string, group models.Group) error {

        v := validator.New()

        _ = v.RegisterValidation("addressrange_valid", func(fl validator.FieldLevel) bool {
		isvalid := functions.IsIpv4CIDR(fl.Field().String())
                return isvalid
        })

        _ = v.RegisterValidation("nameid_valid", func(fl validator.FieldLevel) bool {
		isFieldUnique := operation == "update" || functions.IsGroupNameUnique(fl.Field().String())
		inGroupCharSet := functions.NameInGroupCharSet(fl.Field().String())
		return isFieldUnique && inGroupCharSet
        })

        _ = v.RegisterValidation("displayname_unique", func(fl validator.FieldLevel) bool {
                isFieldUnique := functions.IsGroupDisplayNameUnique(fl.Field().String())
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

//Get number of nodes associated with a group
//May not be necessary, but I think the front end needs it? This should be reviewed after iteration 1
func getGroupNodeNumber(w http.ResponseWriter, r *http.Request) {

        var params = mux.Vars(r)

	count, err := GetGroupNodeNumber(params["groupname"])

        if err != nil {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusInternalServerError, Message: "W1R3: Error retrieving nodes.",
		}
		returnErrorResponse(w, r, errorResponse)
	} else  {
	json.NewEncoder(w).Encode(count)
	}
}

//This is haphazard
//I need a better folder structure
//maybe a functions/ folder and then a node.go, group.go, keys.go, misc.go
func GetGroupNodeNumber(groupName string) (int,  error){

        collection := mongoconn.Client.Database("netmaker").Collection("nodes")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"group": groupName}
        count, err := collection.CountDocuments(ctx, filter)
	returncount := int(count)

	//not sure if this is the right way of handling this error...
        if err != nil {
                return 9999, err
        }

        defer cancel()

        return returncount, err
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
                mongoconn.GetError(err, w)
                return
        }

        json.NewEncoder(w).Encode(group)
}

func keyUpdate(w http.ResponseWriter, r *http.Request) {

        w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

        var group models.Group

        group, err := functions.GetParentGroup(params["groupname"])
        if err != nil {
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

        errN := collection.FindOneAndUpdate(ctx, filter, update).Decode(&group)

        defer cancel()

        if errN != nil {
                mongoconn.GetError(errN, w)
                fmt.Println(errN)
                return
        }

        json.NewEncoder(w).Encode(group)
}

//Update a group
func updateGroup(w http.ResponseWriter, r *http.Request) {

        w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

        var group models.Group

	group, err := functions.GetParentGroup(params["groupname"])
        if err != nil {
                return
        }

        var groupChange models.Group

	haschange := false
	hasrangeupdate := false

	_ = json.NewDecoder(r.Body).Decode(&groupChange)

	if groupChange.AddressRange == "" {
		groupChange.AddressRange = group.AddressRange
	}
	if groupChange.NameID == "" {
		groupChange.NameID =  group.NameID
	}


        err = validateGroup("update", groupChange)
        if err != nil {
                return
        }


	//TODO: group.Name is  not update-able
	//group.Name acts as  the ID for the group and keeps it unique and searchable by nodes
	//should consider renaming to group.ID  
	//Too lazy for now.
	//DisplayName is the editable version and will not be used for node searches,
	//but will be used by front end.

        if groupChange.AddressRange != "" {

            group.AddressRange = groupChange.AddressRange

	    var isAddressOK bool = functions.IsIpv4CIDR(groupChange.AddressRange)
            if !isAddressOK {
                    return
            }
             haschange = true
	     hasrangeupdate = true

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

	//TODO: Important. This doesn't work. This will create cases where we will
	//unintentionally go from allowing manual signup to disallowing
	//need to find a smarter way
	//maybe make into a text field
        if groupChange.AllowManualSignUp != group.AllowManualSignUp {
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
                        {"defaultcheckininterval", group.DefaultCheckInInterval},
		}},
        }

	errN := collection.FindOneAndUpdate(ctx, filter, update).Decode(&group)

        defer cancel()

        if errN != nil {
                mongoconn.GetError(errN, w)
		fmt.Println(errN)
                return
        }

	//Cycles through nodes and gives them new IP's based on the new range
	//Pretty cool, but also pretty inefficient currently
        if hasrangeupdate {
		_ = functions.UpdateGroupNodeAddresses(params["groupname"])
		//json.NewEncoder(w).Encode(errG)
	}
        json.NewEncoder(w).Encode(group)
}

//Delete a group
//Will stop you if  there's any nodes associated
func deleteGroup(w http.ResponseWriter, r *http.Request) {
        // Set header
        w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

        var errorResponse = models.ErrorResponse{
                Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
        }

	nodecount, err := GetGroupNodeNumber(params["groupname"])

	//we dont wanna leave nodes hanging. They need a group!
        if nodecount > 0 || err != nil {
                errorResponse = models.ErrorResponse{
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
                mongoconn.GetError(err, w)
                return
        }

        json.NewEncoder(w).Encode(deleteResult)

}

//Create a group
//Pretty simple
func createGroup(w http.ResponseWriter, r *http.Request) {

        w.Header().Set("Content-Type", "application/json")

	//TODO: 
	//This may be needed to get error response. May be why some errors dont work
	//analyze different error responses and see what needs to be done
	//commenting out for now
	/*
        var errorResponse = models.ErrorResponse{
                Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
        }
	*/
        var group models.Group

        // we decode our body request params
        _ = json.NewDecoder(r.Body).Decode(&group)

	//TODO: Not really doing good validation here. Same as createNode, updateNode, and updateGroup
	//Need to implement some better validation across the board
        err := validateGroup("create", group)
        if err != nil {
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
        _ = result

        defer cancel()

        if err != nil {
                mongoconn.GetError(err, w)
                return
        }
}

// BEGIN KEY MANAGEMENT SECTION
// Consider a separate file for these controllers but I think same file is fine for now


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
                return
        }

        _ = json.NewDecoder(r.Body).Decode(&accesskey)

	if accesskey.Name == "" {
                accesskey.Name = functions.GenKeyName()
        }
	if accesskey.Value == "" {
		accesskey.Value = functions.GenKey()
	}
        if accesskey.Uses == 0 {
                accesskey.Uses = 1
        }
	gconf, errG := functions.GetGlobalConfig()
        if errG != nil {
                mongoconn.GetError(errG, w)
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

        errN := collection.FindOneAndUpdate(ctx, filter, update).Decode(&group)

        defer cancel()

        if errN != nil {
                mongoconn.GetError(errN, w)
                return
        }
	w.Write([]byte(accesskey.Value))
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
                mongoconn.GetError(err, w)
                return
        }
	keydata, keyerr := json.Marshal(group.AccessKeys)

        if keyerr != nil {
                return
        }

	json.Unmarshal(keydata, &keys)

        //json.NewEncoder(w).Encode(group.AccessKeys)
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

        errN := collection.FindOneAndUpdate(ctx, filter, update).Decode(&group)

        defer cancel()

        if errN != nil {
                mongoconn.GetError(errN, w)
                return
        }
}
