package controller

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/serverctl"
)

const ALL_NETWORK_ACCESS = "THIS_USER_HAS_ALL"
const NO_NETWORKS_PRESENT = "THIS_USER_HAS_NONE"

func networkHandlers(r *mux.Router) {
	r.HandleFunc("/api/networks", securityCheck(false, http.HandlerFunc(getNetworks))).Methods("GET")
	r.HandleFunc("/api/networks", securityCheck(true, http.HandlerFunc(createNetwork))).Methods("POST")
	r.HandleFunc("/api/networks/{networkname}", securityCheck(false, http.HandlerFunc(getNetwork))).Methods("GET")
	r.HandleFunc("/api/networks/{networkname}", securityCheck(false, http.HandlerFunc(updateNetwork))).Methods("PUT")
	r.HandleFunc("/api/networks/{networkname}/nodelimit", securityCheck(true, http.HandlerFunc(updateNetworkNodeLimit))).Methods("PUT")
	r.HandleFunc("/api/networks/{networkname}", securityCheck(true, http.HandlerFunc(deleteNetwork))).Methods("DELETE")
	r.HandleFunc("/api/networks/{networkname}/keyupdate", securityCheck(false, http.HandlerFunc(keyUpdate))).Methods("POST")
	r.HandleFunc("/api/networks/{networkname}/keys", securityCheck(false, http.HandlerFunc(createAccessKey))).Methods("POST")
	r.HandleFunc("/api/networks/{networkname}/keys", securityCheck(false, http.HandlerFunc(getAccessKeys))).Methods("GET")
	r.HandleFunc("/api/networks/{networkname}/signuptoken", securityCheck(false, http.HandlerFunc(getSignupToken))).Methods("GET")
	r.HandleFunc("/api/networks/{networkname}/keys/{name}", securityCheck(false, http.HandlerFunc(deleteAccessKey))).Methods("DELETE")
}

//Security check is middleware for every function and just checks to make sure that its the master calling
//Only admin should have access to all these network-level actions
//or maybe some Users once implemented
func securityCheck(reqAdmin bool, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var errorResponse = models.ErrorResponse{
			Code: http.StatusUnauthorized, Message: "W1R3: It's not you it's me.",
		}

		var params = mux.Vars(r)
		bearerToken := r.Header.Get("Authorization")
		err, networks, username := SecurityCheck(reqAdmin, params["networkname"], bearerToken)
		if err != nil {
			if strings.Contains(err.Error(), "does not exist") {
				errorResponse.Code = http.StatusNotFound
			}
			errorResponse.Message = err.Error()
			returnErrorResponse(w, r, errorResponse)
			return
		}
		networksJson, err := json.Marshal(&networks)
		if err != nil {
			errorResponse.Message = err.Error()
			returnErrorResponse(w, r, errorResponse)
			return
		}
		r.Header.Set("user", username)
		r.Header.Set("networks", string(networksJson))
		next.ServeHTTP(w, r)
	}
}

func SecurityCheck(reqAdmin bool, netname string, token string) (error, []string, string) {

	var hasBearer = true
	var tokenSplit = strings.Split(token, " ")
	var authToken = ""

	if len(tokenSplit) < 2 {
		hasBearer = false
	} else {
		authToken = tokenSplit[1]
	}
	userNetworks := []string{}
	//all endpoints here require master so not as complicated
	isMasterAuthenticated := authenticateMaster(authToken)
	username := ""
	if !hasBearer || !isMasterAuthenticated {
		userName, networks, isadmin, err := functions.VerifyUserToken(authToken)
		username = userName
		if err != nil {
			return errors.New("error verifying user token"), nil, username
		}
		if !isadmin && reqAdmin {
			return errors.New("you are unauthorized to access this endpoint"), nil, username
		}
		userNetworks = networks
		if isadmin {
			userNetworks = []string{ALL_NETWORK_ACCESS}
		} else {
			networkexists, err := functions.NetworkExists(netname)
			if err != nil && !database.IsEmptyRecord(err) {
				return err, nil, ""
			}
			if netname != "" && !networkexists {
				return errors.New("this network does not exist"), nil, ""
			}
		}
	} else if isMasterAuthenticated {
		userNetworks = []string{ALL_NETWORK_ACCESS}
	}
	if len(userNetworks) == 0 {
		userNetworks = append(userNetworks, NO_NETWORKS_PRESENT)
	}
	return nil, userNetworks, username
}

//Consider a more secure way of setting master key
func authenticateMaster(tokenString string) bool {
	if tokenString == servercfg.GetMasterKey() {
		return true
	}
	return false
}

//simple get all networks function
func getNetworks(w http.ResponseWriter, r *http.Request) {

	headerNetworks := r.Header.Get("networks")
	networksSlice := []string{}
	marshalErr := json.Unmarshal([]byte(headerNetworks), &networksSlice)
	if marshalErr != nil {
		returnErrorResponse(w, r, formatError(marshalErr, "internal"))
		return
	}
	allnetworks := []models.Network{}
	err := errors.New("Networks Error")
	if networksSlice[0] == ALL_NETWORK_ACCESS {
		allnetworks, err = models.GetNetworks()
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		}
	} else {
		for _, network := range networksSlice {
			netObject, parentErr := functions.GetParentNetwork(network)
			if parentErr == nil {
				allnetworks = append(allnetworks, netObject)
			}
		}
	}
	functions.PrintUserLog(r.Header.Get("user"), "fetched networks.", 2)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(allnetworks)
}

func RemoveComms(networks []models.Network) []models.Network {
	var index int = 100000001
	for ind, net := range networks {
		if net.NetID == "comms" {
			index = ind
		}
	}
	if index == 100000001 {
		return networks
	}
	returnable := make([]models.Network, 0)
	returnable = append(returnable, networks[:index]...)
	return append(returnable, networks[index+1:]...)
}

func ValidateNetworkUpdate(network models.Network) error {
	v := validator.New()

	_ = v.RegisterValidation("netid_valid", func(fl validator.FieldLevel) bool {
		if fl.Field().String() == "" {
			return true
		}
		inCharSet := functions.NameInNetworkCharSet(fl.Field().String())
		return inCharSet
	})

	err := v.Struct(network)

	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			functions.PrintUserLog("validator", e.Error(), 1)
		}
	}
	return err
}

//Simple get network function
func getNetwork(w http.ResponseWriter, r *http.Request) {
	// set header.
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	netname := params["networkname"]
	network, err := GetNetwork(netname)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "fetched network "+netname, 2)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(network)
}

func GetNetwork(name string) (models.Network, error) {
	var network models.Network
	record, err := database.FetchRecord(database.NETWORKS_TABLE_NAME, name)
	if err != nil {
		return network, err
	}

	if err = json.Unmarshal([]byte(record), &network); err != nil {
		return models.Network{}, err
	}

	return network, nil
}

func keyUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	netname := params["networkname"]
	network, err := KeyUpdate(netname)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "updated key on network "+netname, 2)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(network)
}

func KeyUpdate(netname string) (models.Network, error) {
	err := functions.NetworkNodesUpdateAction(netname, models.NODE_UPDATE_KEY)
	if err != nil {
		return models.Network{}, err
	}
	return models.Network{}, nil
}

//Update a network
func AlertNetwork(netid string) error {

	var network models.Network
	network, err := functions.GetParentNetwork(netid)
	if err != nil {
		return err
	}
	updatetime := time.Now().Unix()
	network.NodesLastModified = updatetime
	network.NetworkLastModified = updatetime
	data, err := json.Marshal(&network)
	if err != nil {
		return err
	}
	database.Insert(netid, string(data), database.NETWORKS_TABLE_NAME)
	return nil
}

//Update a network
func updateNetwork(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	var network models.Network
	netname := params["networkname"]
	network, err := functions.GetParentNetwork(netname)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	var newNetwork models.Network
	err = json.NewDecoder(r.Body).Decode(&newNetwork)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	rangeupdate, localrangeupdate, err := network.Update(&newNetwork)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}

	if rangeupdate {
		err = functions.UpdateNetworkNodeAddresses(network.NetID)
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		}
	}
	if localrangeupdate {
		err = functions.UpdateNetworkLocalAddresses(network.NetID)
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "internal"))
			return
		}
	}
	functions.PrintUserLog(r.Header.Get("user"), "updated network "+netname, 1)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newNetwork)
}

func updateNetworkNodeLimit(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	var network models.Network
	netname := params["networkname"]
	network, err := functions.GetParentNetwork(netname)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	var networkChange models.Network

	_ = json.NewDecoder(r.Body).Decode(&networkChange)

	if networkChange.NodeLimit != 0 {
		network.NodeLimit = networkChange.NodeLimit
		data, err := json.Marshal(&network)
		if err != nil {
			returnErrorResponse(w, r, formatError(err, "badrequest"))
			return
		}
		database.Insert(network.NetID, string(data), database.NETWORKS_TABLE_NAME)
		functions.PrintUserLog(r.Header.Get("user"), "updated network node limit on, "+netname, 1)
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(network)
}

//Delete a network
//Will stop you if  there's any nodes associated
func deleteNetwork(w http.ResponseWriter, r *http.Request) {
	// Set header
	w.Header().Set("Content-Type", "application/json")

	var params = mux.Vars(r)
	network := params["networkname"]
	err := DeleteNetwork(network)

	if err != nil {
		errtype := "badrequest"
		if strings.Contains(err.Error(), "Node check failed") {
			errtype = "forbidden"
		}
		returnErrorResponse(w, r, formatError(err, errtype))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "deleted network "+network, 1)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode("success")
}

func DeleteNetwork(network string) error {
	nodeCount, err := functions.GetNetworkNodeCount(network)
	if nodeCount == 0 || database.IsEmptyRecord(err) {
		return database.DeleteRecord(database.NETWORKS_TABLE_NAME, network)
	}
	return errors.New("node check failed. All nodes must be deleted before deleting network")
}

//Create a network
//Pretty simple
func createNetwork(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	var network models.Network

	// we decode our body request params
	err := json.NewDecoder(r.Body).Decode(&network)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}

	err = CreateNetwork(network)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	success, err := serverctl.AddNetwork(network.NetID)
	if err != nil || !success {
		if err == nil {
			err = errors.New("Failed to add server to network " + network.DisplayName)
		}
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "created network "+network.NetID, 1)
	w.WriteHeader(http.StatusOK)
	//json.NewEncoder(w).Encode(result)
}

func CreateNetwork(network models.Network) error {

	network.SetDefaults()
	network.SetNodesLastModified()
	network.SetNetworkLastModified()
	network.KeyUpdateTimeStamp = time.Now().Unix()

	err := network.Validate(false)
	if err != nil {
		//returnErrorResponse(w, r, formatError(err, "badrequest"))
		return err
	}

	data, err := json.Marshal(&network)
	if err != nil {
		return err
	}
	if err = database.Insert(network.NetID, string(data), database.NETWORKS_TABLE_NAME); err != nil {
		return err
	}

	return nil
}

// BEGIN KEY MANAGEMENT SECTION
func createAccessKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	var accesskey models.AccessKey
	//start here
	netname := params["networkname"]
	network, err := functions.GetParentNetwork(netname)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	err = json.NewDecoder(r.Body).Decode(&accesskey)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	key, err := CreateAccessKey(accesskey, network)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "created access key "+accesskey.Name+" on "+netname, 1)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(key)
	//w.Write([]byte(accesskey.AccessString))
}

func CreateAccessKey(accesskey models.AccessKey, network models.Network) (models.AccessKey, error) {

	if accesskey.Name == "" {
		accesskey.Name = functions.GenKeyName()
	}

	if accesskey.Value == "" {
		accesskey.Value = functions.GenKey()
	}
	if accesskey.Uses == 0 {
		accesskey.Uses = 1
	}

	checkkeys, err := GetKeys(network.NetID)
	if err != nil {
		return models.AccessKey{}, errors.New("could not retrieve network keys")
	}

	for _, key := range checkkeys {
		if key.Name == accesskey.Name {
			return models.AccessKey{}, errors.New("duplicate AccessKey Name")
		}
	}
	privAddr := ""
	if network.IsLocal != "" {
		privAddr = network.LocalRange
	}

	netID := network.NetID

	var accessToken models.AccessToken
	s := servercfg.GetServerConfig()
	servervals := models.ServerConfig{
		CoreDNSAddr:    s.CoreDNSAddr,
		APIConnString:  s.APIConnString,
		APIHost:        s.APIHost,
		APIPort:        s.APIPort,
		GRPCConnString: s.GRPCConnString,
		GRPCHost:       s.GRPCHost,
		GRPCPort:       s.GRPCPort,
		GRPCSSL:        s.GRPCSSL,
	}
	accessToken.ServerConfig = servervals
	accessToken.ClientConfig.Network = netID
	accessToken.ClientConfig.Key = accesskey.Value
	accessToken.ClientConfig.LocalRange = privAddr

	tokenjson, err := json.Marshal(accessToken)
	if err != nil {
		return accesskey, err
	}

	accesskey.AccessString = base64.StdEncoding.EncodeToString([]byte(tokenjson))

	//validate accesskey
	v := validator.New()
	err = v.Struct(accesskey)
	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			functions.PrintUserLog("validator", e.Error(), 1)
		}
		return models.AccessKey{}, err
	}

	network.AccessKeys = append(network.AccessKeys, accesskey)
	data, err := json.Marshal(&network)
	if err != nil {
		return models.AccessKey{}, err
	}
	if err = database.Insert(network.NetID, string(data), database.NETWORKS_TABLE_NAME); err != nil {
		return models.AccessKey{}, err
	}

	return accesskey, nil
}

func GetSignupToken(netID string) (models.AccessKey, error) {

	var accesskey models.AccessKey
	var accessToken models.AccessToken
	s := servercfg.GetServerConfig()
	servervals := models.ServerConfig{
		APIConnString:  s.APIConnString,
		APIHost:        s.APIHost,
		APIPort:        s.APIPort,
		GRPCConnString: s.GRPCConnString,
		GRPCHost:       s.GRPCHost,
		GRPCPort:       s.GRPCPort,
		GRPCSSL:        s.GRPCSSL,
	}
	accessToken.ServerConfig = servervals

	tokenjson, err := json.Marshal(accessToken)
	if err != nil {
		return accesskey, err
	}

	accesskey.AccessString = base64.StdEncoding.EncodeToString([]byte(tokenjson))
	return accesskey, nil
}
func getSignupToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	netID := params["networkname"]

	token, err := GetSignupToken(netID)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "got signup token "+netID, 2)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(token)
}

//pretty simple get
func getAccessKeys(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	network := params["networkname"]
	keys, err := GetKeys(network)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "internal"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "fetched access keys on network "+network, 2)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(keys)
}
func GetKeys(net string) ([]models.AccessKey, error) {

	record, err := database.FetchRecord(database.NETWORKS_TABLE_NAME, net)
	if err != nil {
		return []models.AccessKey{}, err
	}
	network, err := functions.ParseNetwork(record)
	if err != nil {
		return []models.AccessKey{}, err
	}
	return network.AccessKeys, nil
}

//delete key. Has to do a little funky logic since it's not a collection item
func deleteAccessKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	keyname := params["name"]
	netname := params["networkname"]
	err := DeleteKey(keyname, netname)
	if err != nil {
		returnErrorResponse(w, r, formatError(err, "badrequest"))
		return
	}
	functions.PrintUserLog(r.Header.Get("user"), "deleted access key "+keyname+" on network "+netname, 1)
	w.WriteHeader(http.StatusOK)
}
func DeleteKey(keyname, netname string) error {
	network, err := functions.GetParentNetwork(netname)
	if err != nil {
		return err
	}
	//basically, turn the list of access keys into the list of access keys before and after the item
	//have not done any error handling for if there's like...1 item. I think it works? need to test.
	found := false
	var updatedKeys []models.AccessKey
	for _, currentkey := range network.AccessKeys {
		if currentkey.Name == keyname {
			found = true
		} else {
			updatedKeys = append(updatedKeys, currentkey)
		}
	}
	if !found {
		return errors.New("key " + keyname + " does not exist")
	}
	network.AccessKeys = updatedKeys
	data, err := json.Marshal(&network)
	if err != nil {
		return err
	}
	if err := database.Insert(network.NetID, string(data), database.NETWORKS_TABLE_NAME); err != nil {
		return err
	}

	return nil
}
