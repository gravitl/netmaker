package controller

import (
    "gopkg.in/go-playground/validator.v9"
    "log"
    "fmt"
    "golang.org/x/crypto/bcrypt"
    "github.com/gravitl/netmaker/mongoconn"
    "github.com/gravitl/netmaker/functions"
    "context"
    "go.mongodb.org/mongo-driver/bson"
    "time"
    "net"
    "github.com/gravitl/netmaker/models"
    "go.mongodb.org/mongo-driver/mongo/options"

)

func GetPeersList(groupName string) ([]models.PeersResponse, error) {

        var peers []models.PeersResponse

        //Connection mongoDB with mongoconn class
        collection := mongoconn.Client.Database("netmaker").Collection("nodes")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        //Get all nodes in the relevant group which are NOT in pending state
        filter := bson.M{"group": groupName, "ispending": false}
        cur, err := collection.Find(ctx, filter)

        if err != nil {
                return peers, err
        }

        // Close the cursor once finished and cancel if it takes too long
        defer cancel()

        for cur.Next(context.TODO()) {

                var peer models.PeersResponse
                err := cur.Decode(&peer)
                if err != nil {
                        log.Fatal(err)
                }

                // add the node to our node array
                //maybe better to just return this? But then that's just GetNodes...
                peers = append(peers, peer)
        }

        //Uh oh, fatal error! This needs some better error handling
        //TODO: needs appropriate error handling so the server doesnt shut down.
        if err := cur.Err(); err != nil {
                log.Fatal(err)
        }

	return peers, err
}


func ValidateNode(operation string, groupName string, node models.Node) error {

        v := validator.New()

        _ = v.RegisterValidation("endpoint_check", func(fl validator.FieldLevel) bool {
                //var isFieldUnique bool = functions.IsFieldUnique(groupName, "endpoint", node.Endpoint)
		isIpv4 := functions.IsIpv4Net(node.Endpoint)
		notEmptyCheck := node.Endpoint != ""
                return (notEmptyCheck && isIpv4) || operation == "update"
        })
        _ = v.RegisterValidation("localaddress_check", func(fl validator.FieldLevel) bool {
                //var isFieldUnique bool = functions.IsFieldUnique(groupName, "endpoint", node.Endpoint)
                isIpv4 := functions.IsIpv4Net(node.LocalAddress)
                notEmptyCheck := node.LocalAddress != ""
                return (notEmptyCheck && isIpv4) || operation == "update"
        })


        _ = v.RegisterValidation("macaddress_unique", func(fl validator.FieldLevel) bool {
                var isFieldUnique bool = functions.IsFieldUnique(groupName, "macaddress", node.MacAddress)
                return isFieldUnique || operation == "update"
        })

        _ = v.RegisterValidation("macaddress_valid", func(fl validator.FieldLevel) bool {
                _, err := net.ParseMAC(node.MacAddress)
                return err == nil
        })

        _ = v.RegisterValidation("name_valid", func(fl validator.FieldLevel) bool {
                isvalid := functions.NameInNodeCharSet(node.Name)
                return isvalid
        })

        _ = v.RegisterValidation("group_exists", func(fl validator.FieldLevel) bool {
		_, err := node.GetGroup()
		return err == nil
        })
        _ = v.RegisterValidation("pubkey_check", func(fl validator.FieldLevel) bool { 
                notEmptyCheck := node.PublicKey != ""
		isBase64 := functions.IsBase64(node.PublicKey)
                return (notEmptyCheck  && isBase64) || operation == "update"
        })
        _ = v.RegisterValidation("password_check", func(fl validator.FieldLevel) bool { 
                notEmptyCheck := node.Password != ""
		goodLength := len(node.Password) > 5
                return (notEmptyCheck && goodLength) || operation == "update"
        })

        err := v.Struct(node)

        if err != nil {
                for _, e := range err.(validator.ValidationErrors) {
                        fmt.Println(e)
                }
        }
        return err
}

func UpdateNode(nodechange models.Node, node models.Node) (models.Node, error) {
    //Question: Is there a better way  of doing  this than a bunch of "if" statements? probably...
    //Eventually, lets have a better way to check if any of the fields are filled out...
    queryMac := node.MacAddress
    notifygroup := false

    if nodechange.Address != "" {
        node.Address = nodechange.Address
	notifygroup = true
    }
    if nodechange.Name != "" {
        node.Name = nodechange.Name
    }
    if nodechange.LocalAddress != "" {
        node.LocalAddress = nodechange.LocalAddress
    }
    if nodechange.ListenPort != 0 {
        node.ListenPort = nodechange.ListenPort
    }
    if nodechange.PreUp != "" {
        node.PreUp = nodechange.PreUp
    }
    if nodechange.Interface != "" {
        node.Interface = nodechange.Interface
    }
    if nodechange.PostUp != "" {
        node.PostUp = nodechange.PostUp
    }
    if nodechange.AccessKey != "" {
        node.AccessKey = nodechange.AccessKey
    }
    if nodechange.Endpoint != "" {
        node.Endpoint = nodechange.Endpoint
	notifygroup = true
}
    if nodechange.SaveConfig != nil {
        node.SaveConfig = nodechange.SaveConfig
    }
    if nodechange.PersistentKeepalive != 0 {
        node.PersistentKeepalive = nodechange.PersistentKeepalive
    }
    if nodechange.Password != "" {
	    err := bcrypt.CompareHashAndPassword([]byte(nodechange.Password), []byte(node.Password))
           if err != nil && nodechange.Password != node.Password {
		hash, err := bcrypt.GenerateFromPassword([]byte(nodechange.Password), 5)
		if err != nil {
			return node, err
		}
		nodechange.Password = string(hash)
		node.Password = nodechange.Password
	}
    }
    if nodechange.MacAddress != "" {
        node.MacAddress = nodechange.MacAddress
    }
    if nodechange.PublicKey != "" {
        node.PublicKey = nodechange.PublicKey
	notifygroup = true
    }

        //collection := mongoconn.ConnectDB()
        collection := mongoconn.Client.Database("netmaker").Collection("nodes")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        // Create filter
        filter := bson.M{"macaddress": queryMac}

        node.SetLastModified()

        // prepare update model.
        update := bson.D{
                {"$set", bson.D{
                        {"name", node.Name},
                        {"password", node.Password},
                        {"listenport", node.ListenPort},
                        {"publickey", node.PublicKey},
                        {"endpoint", node.Endpoint},
                        {"postup", node.PostUp},
                        {"preup", node.PreUp},
                        {"macaddress", node.MacAddress},
                        {"localaddress", node.LocalAddress},
                        {"persistentkeepalive", node.PersistentKeepalive},
                        {"saveconfig", node.SaveConfig},
                        {"accesskey", node.AccessKey},
                        {"interface", node.Interface},
                        {"lastmodified", node.LastModified},
                }},
        }
        var nodeupdate models.Node

        errN := collection.FindOneAndUpdate(ctx, filter, update).Decode(&nodeupdate)
	if errN != nil {
		return nodeupdate, errN
	}

	returnnode, errN := GetNode(node.MacAddress, node.Group)

	defer cancel()

	if notifygroup {
		errN = SetGroupNodesLastModified(node.Group)
	}

	return returnnode, errN
}

func DeleteNode(macaddress string, group string) (bool, error)  {

	deleted := false

        collection := mongoconn.Client.Database("netmaker").Collection("nodes")

	filter := bson.M{"macaddress": macaddress, "group": group}

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        result, err := collection.DeleteOne(ctx, filter)

	deletecount := result.DeletedCount

	if deletecount > 0 {
		deleted = true
	}

        defer cancel()

        err = SetGroupNodesLastModified(group)
	fmt.Println("Deleted node " + macaddress + " from group " + group)

	return deleted, err
}

func GetNode(macaddress string, group string) (models.Node, error) {

        var node models.Node

        collection := mongoconn.Client.Database("netmaker").Collection("nodes")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"macaddress": macaddress, "group": group}
        err := collection.FindOne(ctx, filter, options.FindOne().SetProjection(bson.M{"_id": 0})).Decode(&node)

        defer cancel()

	return node, err
}

func CreateNode(node models.Node, groupName string) (models.Node, error) {

        //encrypt that password so we never see it again
        hash, err := bcrypt.GenerateFromPassword([]byte(node.Password), 5)

        if err != nil {
                return node, err
        }
        //set password to encrypted password
        node.Password = string(hash)


        node.Group = groupName

        //node.SetDefaults()
        //Umm, why am I doing this again?
        //TODO: Why am I using a local function instead of the struct function? I really dont know.
        //I think I thought it didn't work but uhhh...idk
        //anyways, this sets some sensible variables for unset params.
        node.SetDefaults()

        //Another DB call here...Inefficient
        //Anyways, this scrolls through all the IP Addresses in the group range and checks against nodes
        //until one is open and then returns it
        node.Address, err = functions.UniqueAddress(groupName)

        if err != nil {/*
		errorResponse := models.ErrorResponse{
                        Code: http.StatusInternalServerError, Message: "W1R3: Encountered an internal error! ",
                }*/
                //returnErrorResponse(w, r, errorResponse)
                return node, err
        }

        //IDK why these aren't a part of "set defaults. Pretty dumb.
        //TODO: This is dumb. Consolidate and fix.
        node.SetLastModified()
        node.SetDefaultName()
	node.SetLastCheckIn()
	node.SetLastPeerUpdate()


        //Create a JWT for the node
        tokenString, _ := functions.CreateJWT(node.MacAddress, groupName)

        if tokenString == "" {
                //returnErrorResponse(w, r, errorResponse)
		return node, err
        }

        // connect db
        collection := mongoconn.Client.Database("netmaker").Collection("nodes")
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)


        // insert our node to the node db.
        result, err := collection.InsertOne(ctx, node)
        _ = result

        defer cancel()

        if err != nil {
                return node, err
        }
        //return response for if node  is pending
        if !node.IsPending {

        functions.DecrimentKey(node.Group, node.AccessKey)

        }

	SetGroupNodesLastModified(node.Group)

        return node, err
}

func NodeCheckIn(node models.Node, groupName string) (models.CheckInResponse, error) {

	var response models.CheckInResponse

	parentgroup, err := functions.GetParentGroup(groupName)
        if err != nil{
		err = fmt.Errorf("%w; Couldnt retrieve Group " + groupName  + ": ", err)
                return response, err
        }

	parentnode, err := functions.GetNodeByMacAddress(groupName, node.MacAddress)
        if err != nil{
		err = fmt.Errorf("%w; Couldnt Get Node " + node.MacAddress, err)
                return response, err
        }
	if parentnode.IsPending {
                err = fmt.Errorf("%w; Node checking in is still pending: " + node.MacAddress, err)
		response.IsPending = true
		return response, err
	}

        grouplm := parentgroup.GroupLastModified
	peerslm := parentgroup.NodesLastModified
        peerlistlm := parentnode.LastPeerUpdate
        parentnodelm := parentnode.LastModified
	parentnodelastcheckin := parentnode.LastCheckIn

	if parentnodelastcheckin < parentnodelm {
		response.NeedConfigUpdate = true
	}

	if parentnodelm < grouplm {
		response.NeedConfigUpdate = true
	}
	if peerlistlm < peerslm {
		response.NeedPeerUpdate = true
	}
	/*
	if postchanges {
		parentnode, err = UpdateNode(node, parentnode)
	        if err != nil{
			err = fmt.Errorf("%w; Couldnt Update Node: ", err)
			return response, err
		} else {
			response.NodeUpdated = true
		}
	}
	*/
	err = TimestampNode(parentnode, true,  false, false)

	if err != nil{
		err = fmt.Errorf("%w; Couldnt Timestamp Node: ", err)
		return response, err
	}
	response.Success = true

        return response, err
}

func SetGroupNodesLastModified(groupName string) error {

	timestamp := time.Now().Unix()

        collection := mongoconn.Client.Database("netmaker").Collection("groups")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        // Create filter
        filter := bson.M{"nameid": groupName}

        // prepare update model.
        update := bson.D{
                {"$set", bson.D{
                        {"nodeslastmodified", timestamp},
                }},
        }

        result := collection.FindOneAndUpdate(ctx, filter, update)

        defer cancel()

	if result.Err() != nil {
                return result.Err()
        }

        return nil
}

func TimestampNode(node models.Node, updatecheckin bool, updatepeers bool, updatelm bool) error{
        if updatelm {
		node.SetLastModified()
	}
	if updatecheckin {
		node.SetLastCheckIn()
	}
	if updatepeers {
		node.SetLastPeerUpdate()
	}

        collection := mongoconn.Client.Database("netmaker").Collection("nodes")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        // Create filter
        filter := bson.M{"macaddress": node.MacAddress}

        // prepare update model.
        update := bson.D{
                {"$set", bson.D{
                        {"lastmodified", node.LastModified},
                        {"lastpeerupdate", node.LastPeerUpdate},
                        {"lastcheckin", node.LastCheckIn},
                }},
        }

	var nodeupdate models.Node
        err := collection.FindOneAndUpdate(ctx, filter, update).Decode(&nodeupdate)
        defer cancel()

	return err
}


