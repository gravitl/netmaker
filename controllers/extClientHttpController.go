package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	// "fmt"
	"net/http"
	"time"
	"strconv"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mongoconn"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"github.com/skip2/go-qrcode"
)

func extClientHandlers(r *mux.Router) {

	r.HandleFunc("/api/extclients", securityCheck(http.HandlerFunc(getAllExtClients))).Methods("GET")
	r.HandleFunc("/api/extclients/{network}", securityCheck(http.HandlerFunc(getNetworkExtClients))).Methods("GET")
	r.HandleFunc("/api/extclients/{network}/{clientid}", securityCheck(http.HandlerFunc(getExtClient))).Methods("GET")
	r.HandleFunc("/api/extclients/{network}/{clientid}/{type}", securityCheck(http.HandlerFunc(getExtClientConf))).Methods("GET")
	r.HandleFunc("/api/extclients/{network}/{clientid}", securityCheck(http.HandlerFunc(updateExtClient))).Methods("PUT")
	r.HandleFunc("/api/extclients/{network}/{clientid}", securityCheck(http.HandlerFunc(deleteExtClient))).Methods("DELETE")
	r.HandleFunc("/api/extclients/{network}/{macaddress}", securityCheck(http.HandlerFunc(createExtClient))).Methods("POST")
}

// TODO: Implement Validation
func ValidateExtClientCreate(networkName string, extclient models.ExtClient) error {
	// 	v := validator.New()
	// 	_ = v.RegisterValidation("macaddress_unique", func(fl validator.FieldLevel) bool {
	// 		var isFieldUnique bool = functions.IsFieldUnique(networkName, "macaddress", extclient.MacAddress)
	// 		return isFieldUnique
	// 	})
	// 	_ = v.RegisterValidation("network_exists", func(fl validator.FieldLevel) bool {
	// 		_, err := extclient.GetNetwork()
	// 		return err == nil
	// 	})
	// 	err := v.Struct(extclient)

	// 	if err != nil {
	// 		for _, e := range err.(validator.ValidationErrors) {
	// 			fmt.Println(e)
	// 		}
	// 	}
	return nil
}

// TODO: Implement Validation
func ValidateExtClientUpdate(networkName string, extclient models.ExtClient) error {
	// v := validator.New()
	// _ = v.RegisterValidation("network_exists", func(fl validator.FieldLevel) bool {
	// 	_, err := extclient.GetNetwork()
	// 	return err == nil
	// })
	// err := v.Struct(extclient)
	// if err != nil {
	// 	for _, e := range err.(validator.ValidationErrors) {
	// 		fmt.Println(e)
	// 	}
	// }
	return nil
}

func checkIngressExists(network string, macaddress string) bool {
	node, err := functions.GetNodeByMacAddress(network, macaddress)
	if err != nil {
		return false
	}
	return node.IsIngressGateway
}

//Gets all extclients associated with network, including pending extclients
func getNetworkExtClients(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var extclients []models.ExtClient
	var params = mux.Vars(r)
	extclients, err := GetNetworkExtClients(params["network"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	//Returns all the extclients in JSON format
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(extclients)
}

func GetNetworkExtClients(network string) ([]models.ExtClient, error) {
	var extclients []models.ExtClient
	collection := mongoconn.Client.Database("netmaker").Collection("extclients")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	filter := bson.M{"network": network}
	//Filtering out the ID field cuz Dillon doesn't like it. May want to filter out other fields in the future
	cur, err := collection.Find(ctx, filter, options.Find().SetProjection(bson.M{"_id": 0}))
	if err != nil {
		return []models.ExtClient{}, err
	}
	defer cancel()
	for cur.Next(context.TODO()) {
		//Using a different model for the ReturnExtClient (other than regular extclient).
		//Either we should do this for ALL structs (so Networks and Keys)
		//OR we should just use the original struct
		//My preference is to make some new return structs
		//TODO: Think about this. Not an immediate concern. Just need to get some consistency eventually
		var extclient models.ExtClient
		err := cur.Decode(&extclient)
		if err != nil {
			return []models.ExtClient{}, err
		}
		// add item our array of extclients
		extclients = append(extclients, extclient)
	}
	//TODO: Another fatal error we should take care of.
	if err := cur.Err(); err != nil {
		return []models.ExtClient{}, err
	}
	return extclients, nil
}

//A separate function to get all extclients, not just extclients for a particular network.
//Not quite sure if this is necessary. Probably necessary based on front end but may want to review after iteration 1 if it's being used or not
func getAllExtClients(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	extclients, err := functions.GetAllExtClients()
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	//Return all the extclients in JSON format
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(extclients)
}

//Get an individual extclient. Nothin fancy here folks.
func getExtClient(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	var extclient models.ExtClient

	collection := mongoconn.Client.Database("netmaker").Collection("extclients")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	filter := bson.M{"network": params["network"], "clientid": params["clientid"]}
	err := collection.FindOne(ctx, filter, options.FindOne().SetProjection(bson.M{"_id": 0})).Decode(&extclient)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	defer cancel()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(extclient)
}

//Get an individual extclient. Nothin fancy here folks.
func getExtClientConf(w http.ResponseWriter, r *http.Request) {
        // set header.
        w.Header().Set("Content-Type", "application/json")

        var params = mux.Vars(r)

        var extclient models.ExtClient

        collection := mongoconn.Client.Database("netmaker").Collection("extclients")

        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        filter := bson.M{"network": params["network"], "clientid": params["clientid"]}
        err := collection.FindOne(ctx, filter, options.FindOne().SetProjection(bson.M{"_id": 0})).Decode(&extclient)
        if err != nil {
                returnErrorResponse(w, r, formatError(err, "internal"))
                return
        }

        gwnode, err := functions.GetNodeByMacAddress(extclient.Network, extclient.IngressGatewayID)
        if err != nil {
		fmt.Println("Could not retrieve Ingress Gateway Node " + extclient.IngressGatewayID)
		returnErrorResponse(w, r, formatError(err, "internal"))
                return
        }

	network, err := functions.GetParentNetwork(extclient.Network)
        if err != nil {
                fmt.Println("Could not retrieve Ingress Gateway Network " + extclient.Network)
                returnErrorResponse(w, r, formatError(err, "internal"))
                return
        }
	keepalive := ""
	if network.DefaultKeepalive != 0 {
		keepalive = "PersistentKeepalive = " + strconv.Itoa(int(network.DefaultKeepalive))
	}
	gwendpoint := gwnode.Endpoint + ":" + strconv.Itoa(int(gwnode.ListenPort))
	config := fmt.Sprintf(`[Interface]
Address = %s
PrivateKey = %s

[Peer]
PublicKey = %s
AllowedIPs = %s
Endpoint = %s
%s

`, extclient.Address + "/32",
   extclient.PrivateKey,
   gwnode.PublicKey,
   network.AddressRange,
   gwendpoint,
   keepalive)

	if params["type"] == "qr" {
		bytes, err := qrcode.Encode(config, qrcode.Medium, 220)
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(bytes)
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		}
		return
	}

	if params["type"] == "file" {
		name := extclient.ClientID + ".conf"
                w.Header().Set("Content-Type", "application/config")
		w.Header().Set("Content-Disposition", "attachment; filename=\"" + name + "\"")
		w.WriteHeader(http.StatusOK)
		_, err := fmt.Fprint(w, config)
		if err != nil {
                        returnErrorResponse(w, r, formatError(err, "internal"))
		}
		return
	}

        defer cancel()

        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(extclient)
}

func CreateExtClient(extclient models.ExtClient) error {
	fmt.Println(extclient)
	// Generate Private Key for new ExtClient
	if extclient.PrivateKey == "" {
		privateKey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return err
		}

		extclient.PrivateKey = privateKey.String()
		extclient.PublicKey = privateKey.PublicKey().String()
	}

	if extclient.Address == "" {
		newAddress, err := functions.UniqueAddress(extclient.Network)
		if err != nil {
			return err
		}
		extclient.Address = newAddress
	}

        if extclient.ClientID == "" {
                cid := StringWithCharset(7, charset)
                clientid := "client-" + cid
                extclient.ClientID = clientid
        }

	extclient.LastModified = time.Now().Unix()

	collection := mongoconn.Client.Database("netmaker").Collection("extclients")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// insert our network into the network table
	_, err := collection.InsertOne(ctx, extclient)
	defer cancel()
	return err
}

//This one's a doozy
//To create a extclient
//Must have valid key and be unique
func createExtClient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	networkName := params["network"]
	macaddress := params["macaddress"]
	//Check if network exists  first
	//TODO: This is inefficient. Let's find a better way.
	//Just a few rows down we grab the network anyway
	ingressExists := checkIngressExists(networkName, macaddress)
	if !ingressExists {
		returnErrorResponse(w, r, formatError(errors.New("ingress does not exist"), "internal"))
		return
	}

	var extclient models.ExtClient
	extclient.Network = networkName
	extclient.IngressGatewayID = macaddress

	//get extclient from body of request
	err := json.NewDecoder(r.Body).Decode(&extclient)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	err = ValidateExtClientCreate(params["network"], extclient)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}

	err = CreateExtClient(extclient)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	w.WriteHeader(http.StatusOK)
}

func updateExtClient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	var newExtClient models.ExtClient
	var oldExtClient models.ExtClient
	// we decode our body request params
	_ = json.NewDecoder(r.Body).Decode(&newExtClient)
	// TODO: Validation for update.
	// err := ValidateExtClientUpdate(params["network"], params["clientid"], newExtClient)
	// if err != nil {
	// 	returnErrorResponse(w, r, formatError(err, "badrequest"))
	// 	return
	// }
	collection := mongoconn.Client.Database("netmaker").Collection("extclients")
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	filter := bson.M{"network": params["network"], "clientid": params["clientid"]}
	err := collection.FindOne(ctx, filter, options.FindOne().SetProjection(bson.M{"_id": 0})).Decode(&oldExtClient)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	success, err := DeleteExtClient(params["network"], params["clientid"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	} else if !success {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	oldExtClient.ClientID = newExtClient.ClientID
	CreateExtClient(oldExtClient)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(oldExtClient)
}

func DeleteExtClient(network string, clientid string) (bool, error) {

	deleted := false

	collection := mongoconn.Client.Database("netmaker").Collection("extclients")

	filter := bson.M{"network": network, "clientid": clientid}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	result, err := collection.DeleteOne(ctx, filter)

	deletecount := result.DeletedCount

	if deletecount > 0 {
		deleted = true
	}

	defer cancel()

	fmt.Println("Deleted extclient client " + clientid + " from network " + network)
	return deleted, err
}

//Delete a extclient
//Pretty straightforward
func deleteExtClient(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params
	var params = mux.Vars(r)

	success, err := DeleteExtClient(params["network"], params["clientid"])

	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	} else if !success {
		err = errors.New("Could not delete extclient " + params["clientid"])
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	returnSuccessResponse(w, r, params["clientid"]+" deleted.")
}

func StringWithCharset(length int, charset string) string {
        b := make([]byte, length)
        for i := range b {
                b[i] = charset[seededRand.Intn(len(charset))]
        }
        return string(b)
}

