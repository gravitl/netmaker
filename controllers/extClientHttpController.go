package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/skip2/go-qrcode"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func extClientHandlers(r *mux.Router) {

	r.HandleFunc("/api/extclients", securityCheck(true, http.HandlerFunc(getAllExtClients))).Methods("GET")
	r.HandleFunc("/api/extclients/{network}", securityCheck(false, http.HandlerFunc(getNetworkExtClients))).Methods("GET")
	r.HandleFunc("/api/extclients/{network}/{clientid}", securityCheck(false, http.HandlerFunc(getExtClient))).Methods("GET")
	r.HandleFunc("/api/extclients/{network}/{clientid}/{type}", securityCheck(false, http.HandlerFunc(getExtClientConf))).Methods("GET")
	r.HandleFunc("/api/extclients/{network}/{clientid}", securityCheck(false, http.HandlerFunc(updateExtClient))).Methods("PUT")
	r.HandleFunc("/api/extclients/{network}/{clientid}", securityCheck(false, http.HandlerFunc(deleteExtClient))).Methods("DELETE")
	r.HandleFunc("/api/extclients/{network}/{macaddress}", securityCheck(false, http.HandlerFunc(createExtClient))).Methods("POST")
}

func checkIngressExists(network string, macaddress string) bool {
	node, err := functions.GetNodeByMacAddress(network, macaddress)
	if err != nil {
		return false
	}
	return node.IsIngressGateway == "yes"
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

	records, err := database.FetchRecords(database.EXT_CLIENT_TABLE_NAME)
	if err != nil {
		return extclients, err
	}
	for _, value := range records {
		var extclient models.ExtClient
		err = json.Unmarshal([]byte(value), &extclient)
		if err != nil {
			continue
		}
		if extclient.Network == network {
			extclients = append(extclients, extclient)
		}
	}
	return extclients, err
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

	clientid := params["clientid"]
	network := params["network"]
	client, err := GetExtClient(clientid, network)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(client)
}

func GetExtClient(clientid string, network string) (models.ExtClient, error) {
	var extclient models.ExtClient
	key, err := functions.GetRecordKey(clientid, network)
	if err != nil {
		return extclient, err
	}
	data, err := database.FetchRecord(database.EXT_CLIENT_TABLE_NAME, key)
	if err != nil {
		return extclient, err
	}
	err = json.Unmarshal([]byte(data), &extclient)

	return extclient, err
}

//Get an individual extclient. Nothin fancy here folks.
func getExtClientConf(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	clientid := params["clientid"]
	networkid := params["network"]
	client, err := GetExtClient(clientid, networkid)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	gwnode, err := functions.GetNodeByMacAddress(client.Network, client.IngressGatewayID)
	if err != nil {
		functions.PrintUserLog(r.Header.Get("user"),"Could not retrieve Ingress Gateway Node " + client.IngressGatewayID,1)
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	network, err := functions.GetParentNetwork(client.Network)
	if err != nil {
		functions.PrintUserLog(r.Header.Get("user"),"Could not retrieve Ingress Gateway Network " + client.Network,1)
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

`, client.Address+"/32",
		client.PrivateKey,
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
		name := client.ClientID + ".conf"
		w.Header().Set("Content-Type", "application/config")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
		w.WriteHeader(http.StatusOK)
		_, err := fmt.Fprint(w, config)
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
		}
		return
	}
	functions.PrintUserLog(r.Header.Get("user"),"retrieved ext client config",2)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(client)
}

func CreateExtClient(extclient models.ExtClient) error {
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
		extclient.ClientID = models.GenerateNodeName()
	}

	extclient.LastModified = time.Now().Unix()

	key, err := functions.GetRecordKey(extclient.ClientID, extclient.Network)
	if err != nil {
		return err
	}
	data, err := json.Marshal(&extclient)
	if err != nil {
		return err
	}
	if err = database.Insert(key, string(data), database.EXT_CLIENT_TABLE_NAME); err != nil {
		return err
	}
	err = SetNetworkNodesLastModified(extclient.Network)
	return err
}
/**
 * To create a extclient
 * Must have valid key and be unique
 */
func createExtClient(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)

	networkName := params["network"]
	macaddress := params["macaddress"]
	ingressExists := checkIngressExists(networkName, macaddress)
	if !ingressExists {
		returnErrorResponse(w, r, formatError(errors.New("ingress does not exist"), "internal"))
		return
	}

	var extclient models.ExtClient
	extclient.Network = networkName
	extclient.IngressGatewayID = macaddress
	node, err := functions.GetNodeByMacAddress(networkName, macaddress)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	extclient.IngressGatewayEndpoint = node.Endpoint + ":" + strconv.FormatInt(int64(node.ListenPort), 10)

	err = json.NewDecoder(r.Body).Decode(&extclient)
	if err != nil && !errors.Is(err, io.EOF) {
		returnErrorResponse(w, r, formatError(err, "internal"))
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
	_ = json.NewDecoder(r.Body).Decode(&newExtClient)

	key, err := functions.GetRecordKey(params["clientid"], params["network"])
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	data, err := database.FetchRecord(database.EXT_CLIENT_TABLE_NAME, key)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	if err = json.Unmarshal([]byte(data), &oldExtClient); err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	newclient, err := UpdateExtClient(newExtClient.ClientID, params["network"], oldExtClient)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "updated client "+newExtClient.ClientID, 1)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newclient)
}

func UpdateExtClient(newclientid string, network string, client models.ExtClient) (models.ExtClient, error) {

	err := DeleteExtClient(network, client.ClientID)
	if err != nil {
		return client, err
	}
	client.ClientID = newclientid
	CreateExtClient(client)
	return client, err
}

func DeleteExtClient(network string, clientid string) error {
	key, err := functions.GetRecordKey(clientid, network)
	if err != nil {
		return err
	}
	err = database.DeleteRecord(database.EXT_CLIENT_TABLE_NAME, key)
	return err
}

/**
 * Deletes ext clients based on gateway (mac) of ingress node and network
 */
func DeleteGatewayExtClients(gatewayID string, networkName string) error {
	currentExtClients, err := GetNetworkExtClients(networkName)
	if err != nil {
		return err
	}
	for _, extClient := range currentExtClients {
		if extClient.IngressGatewayID == gatewayID {
			if err = DeleteExtClient(networkName, extClient.ClientID); err != nil {
				functions.PrintUserLog(models.NODE_SERVER_NAME, "failed to remove ext client "+extClient.ClientID, 2)
				continue
			}
		}
	}
	return nil
}

//Delete a extclient
//Pretty straightforward
func deleteExtClient(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	// get params
	var params = mux.Vars(r)

	err := DeleteExtClient(params["network"], params["clientid"])

	if err != nil {
		err = errors.New("Could not delete extclient " + params["clientid"])
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"),
		"Deleted extclient client "+params["clientid"]+" from network "+params["network"], 1)
	returnSuccessResponse(w, r, params["clientid"]+" deleted.")
}

func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

const charset = "abcdefghijklmnopqrstuvwxyz" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))
