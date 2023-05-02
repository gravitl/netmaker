package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/hostactions"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/crypto/bcrypt"
)

func hostHandlers(r *mux.Router) {
	r.HandleFunc("/api/hosts", logic.SecurityCheck(true, http.HandlerFunc(getHosts))).Methods(http.MethodGet)
	r.HandleFunc("/api/hosts/keys", logic.SecurityCheck(true, http.HandlerFunc(updateAllKeys))).Methods(http.MethodPut)
	r.HandleFunc("/api/hosts/{hostid}/keys", logic.SecurityCheck(true, http.HandlerFunc(updateKeys))).Methods(http.MethodPut)
	r.HandleFunc("/api/hosts/{hostid}", logic.SecurityCheck(true, http.HandlerFunc(updateHost))).Methods(http.MethodPut)
	r.HandleFunc("/api/hosts/{hostid}", logic.SecurityCheck(true, http.HandlerFunc(deleteHost))).Methods(http.MethodDelete)
	r.HandleFunc("/api/hosts/{hostid}/networks/{network}", logic.SecurityCheck(true, http.HandlerFunc(addHostToNetwork))).Methods(http.MethodPost)
	r.HandleFunc("/api/hosts/{hostid}/networks/{network}", logic.SecurityCheck(true, http.HandlerFunc(deleteHostFromNetwork))).Methods(http.MethodDelete)
	r.HandleFunc("/api/hosts/{hostid}/relay", logic.SecurityCheck(false, http.HandlerFunc(createHostRelay))).Methods(http.MethodPost)
	r.HandleFunc("/api/hosts/{hostid}/relay", logic.SecurityCheck(false, http.HandlerFunc(deleteHostRelay))).Methods(http.MethodDelete)
	r.HandleFunc("/api/hosts/adm/authenticate", authenticateHost).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/host", authorize(true, false, "host", http.HandlerFunc(pull))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/auth-register/host", socketHandler)
}

// swagger:route GET /api/hosts hosts getHosts
//
// Lists all hosts.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: getHostsSliceResponse
func getHosts(w http.ResponseWriter, r *http.Request) {
	currentHosts, err := logic.GetAllHosts()
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to fetch hosts: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	// return JSON/API formatted hosts
	apiHosts := logic.GetAllHostsAPI(currentHosts[:])
	logger.Log(2, r.Header.Get("user"), "fetched all hosts")
	logic.SortApiHosts(apiHosts[:])
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiHosts)
}

// swagger:route GET /api/v1/host pull pullHost
//
// Used by clients for "pull" command
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: pull
func pull(w http.ResponseWriter, r *http.Request) {

	hostID := r.Header.Get(hostIDHeader) // return JSON/API formatted keys
	if len(hostID) == 0 {
		logger.Log(0, "no host authorized to pull")
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("no host authorized to pull"), "internal"))
		return
	}
	host, err := logic.GetHost(hostID)
	if err != nil {
		logger.Log(0, "no host found during pull", hostID)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	hPU, err := logic.GetPeerUpdateForHost(context.Background(), "", host, nil, nil)
	if err != nil {
		logger.Log(0, "could not pull peers for host", hostID)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	serverConf := servercfg.GetServerInfo()
	if servercfg.GetBrokerType() == servercfg.EmqxBrokerType {
		serverConf.MQUserName = hostID
	}
	response := models.HostPull{
		Host:         *host,
		ServerConfig: serverConf,
		Peers:        hPU.Peers,
		PeerIDs:      hPU.PeerIDs,
	}

	logger.Log(1, hostID, "completed a pull")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&response)
}

// swagger:route PUT /api/hosts/{hostid} hosts updateHost
//
// Updates a Netclient host on Netmaker server.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: updateHostResponse
func updateHost(w http.ResponseWriter, r *http.Request) {
	var newHostData models.ApiHost
	err := json.NewDecoder(r.Body).Decode(&newHostData)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	// confirm host exists
	currHost, err := logic.GetHost(newHostData.ID)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	newHost := newHostData.ConvertAPIHostToNMHost(currHost)
	// check if relay information is changed
	updateRelay := false
	if newHost.IsRelay && len(newHost.RelayedHosts) > 0 {
		if len(newHost.RelayedHosts) != len(currHost.RelayedHosts) || !reflect.DeepEqual(newHost.RelayedHosts, currHost.RelayedHosts) {
			updateRelay = true
		}
	}

	logic.UpdateHost(newHost, currHost) // update the in memory struct values
	if err = logic.UpsertHost(newHost); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to update a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if updateRelay {
		logic.UpdateHostRelay(currHost.ID.String(), currHost.RelayedHosts, newHost.RelayedHosts)
	}
	// publish host update through MQ
	if err := mq.HostUpdate(&models.HostUpdate{
		Action: models.UpdateHost,
		Host:   *newHost,
	}); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to send host update: ", currHost.ID.String(), err.Error())
	}
	go func() {
		if err := mq.PublishPeerUpdate(); err != nil {
			logger.Log(0, "fail to publish peer update: ", err.Error())
		}
		if newHost.Name != currHost.Name {
			networks := logic.GetHostNetworks(currHost.ID.String())
			if err := mq.PublishHostDNSUpdate(currHost, newHost, networks); err != nil {
				var dnsError *models.DNSError
				if errors.Is(err, dnsError) {
					for _, message := range err.(models.DNSError).ErrorStrings {
						logger.Log(0, message)
					}
				} else {
					logger.Log(0, err.Error())
				}
			}
		}
	}()

	apiHostData := newHost.ConvertNMHostToAPI()
	logger.Log(2, r.Header.Get("user"), "updated host", newHost.ID.String())
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiHostData)
}

// swagger:route DELETE /api/hosts/{hostid} hosts deleteHost
//
// Deletes a Netclient host from Netmaker server.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: deleteHostResponse
func deleteHost(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	hostid := params["hostid"]
	// confirm host exists
	currHost, err := logic.GetHost(hostid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to delete a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if currHost.IsRelay {
		if _, _, err := logic.DeleteHostRelay(hostid); err != nil {
			logger.Log(0, r.Header.Get("user"), "failed to dissociate host from relays:", err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	}
	if currHost.IsRelayed {
		relayHost, err := logic.GetHost(currHost.RelayedBy)
		if err != nil {
			logger.Log(0, r.Header.Get("user"), "failed to fetch relay host:", err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		newRelayedHosts := make([]string, 0)
		for _, relayedHostID := range relayHost.RelayedHosts {
			if relayedHostID != hostid {
				newRelayedHosts = append(newRelayedHosts, relayedHostID)
			}
		}
		relayHost.RelayedHosts = newRelayedHosts
		if err := logic.UpsertHost(relayHost); err != nil {
			logger.Log(0, r.Header.Get("user"), "failed to update host relays:", err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	}
	if err = logic.RemoveHost(currHost); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to delete a host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	if err = mq.HostUpdate(&models.HostUpdate{
		Action: models.DeleteHost,
		Host:   *currHost,
	}); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to send delete host update: ", currHost.ID.String(), err.Error())
	}

	apiHostData := currHost.ConvertNMHostToAPI()
	logger.Log(2, r.Header.Get("user"), "removed host", currHost.Name)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(apiHostData)
}

// swagger:route POST /api/hosts/{hostid}/networks/{network} hosts addHostToNetwork
//
// Given a network, a host is added to the network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: addHostToNetworkResponse
func addHostToNetwork(w http.ResponseWriter, r *http.Request) {

	var params = mux.Vars(r)
	hostid := params["hostid"]
	network := params["network"]
	if hostid == "" || network == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("hostid or network cannot be empty"), "badrequest"))
		return
	}
	// confirm host exists
	currHost, err := logic.GetHost(hostid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to find host:", hostid, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	newNode, err := logic.UpdateHostNetwork(currHost, network, true)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to add host to network:", hostid, network, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(1, "added new node", newNode.ID.String(), "to host", currHost.Name)
	hostactions.AddAction(models.HostUpdate{
		Action: models.JoinHostToNetwork,
		Host:   *currHost,
		Node:   *newNode,
	})
	if servercfg.IsMessageQueueBackend() {
		mq.HostUpdate(&models.HostUpdate{
			Action: models.RequestAck,
			Host:   *currHost,
		})
	}

	logger.Log(2, r.Header.Get("user"), fmt.Sprintf("added host %s to network %s", currHost.Name, network))
	w.WriteHeader(http.StatusOK)
}

// swagger:route DELETE /api/hosts/{hostid}/networks/{network} hosts deleteHostFromNetwork
//
// Given a network, a host is removed from the network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: deleteHostFromNetworkResponse
func deleteHostFromNetwork(w http.ResponseWriter, r *http.Request) {

	var params = mux.Vars(r)
	hostid := params["hostid"]
	network := params["network"]
	if hostid == "" || network == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("hostid or network cannot be empty"), "badrequest"))
		return
	}
	// confirm host exists
	currHost, err := logic.GetHost(hostid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to find host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	node, err := logic.UpdateHostNetwork(currHost, network, false)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to remove host from network:", hostid, network, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	node.Action = models.NODE_DELETE
	node.PendingDelete = true
	logger.Log(1, "deleting  node", node.ID.String(), "from host", currHost.Name)
	if err := logic.DeleteNode(node, false); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("failed to delete node"), "internal"))
		return
	}
	// notify node change

	runUpdates(node, false)
	go func() { // notify of peer change
		if err := mq.PublishPeerUpdate(); err != nil {
			logger.Log(1, "error publishing peer update ", err.Error())
		}
		if err := mq.PublishDNSDelete(node, currHost); err != nil {
			logger.Log(1, "error publishing dns update", err.Error())
		}
	}()
	logger.Log(2, r.Header.Get("user"), fmt.Sprintf("removed host %s from network %s", currHost.Name, network))
	w.WriteHeader(http.StatusOK)
}

// swagger:route POST /api/hosts/adm/authenticate hosts authenticateHost
//
// Host based authentication for making further API calls.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: successResponse
func authenticateHost(response http.ResponseWriter, request *http.Request) {
	var authRequest models.AuthParams
	var errorResponse = models.ErrorResponse{
		Code: http.StatusInternalServerError, Message: "W1R3: It's not you it's me.",
	}

	decoder := json.NewDecoder(request.Body)
	decoderErr := decoder.Decode(&authRequest)
	defer request.Body.Close()

	if decoderErr != nil {
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = decoderErr.Error()
		logger.Log(0, request.Header.Get("user"), "error decoding request body: ",
			decoderErr.Error())
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}
	errorResponse.Code = http.StatusBadRequest
	if authRequest.ID == "" {
		errorResponse.Message = "W1R3: ID can't be empty"
		logger.Log(0, request.Header.Get("user"), errorResponse.Message)
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	} else if authRequest.Password == "" {
		errorResponse.Message = "W1R3: Password can't be empty"
		logger.Log(0, request.Header.Get("user"), errorResponse.Message)
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}
	host, err := logic.GetHost(authRequest.ID)
	if err != nil {
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logger.Log(0, request.Header.Get("user"),
			"error retrieving host: ", err.Error())
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}
	err = bcrypt.CompareHashAndPassword([]byte(host.HostPass), []byte(authRequest.Password))
	if err != nil {
		errorResponse.Code = http.StatusUnauthorized
		errorResponse.Message = "unauthorized"
		logger.Log(0, request.Header.Get("user"),
			"error validating user password: ", err.Error())
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}

	tokenString, err := logic.CreateJWT(authRequest.ID, authRequest.MacAddress, "")
	if tokenString == "" {
		errorResponse.Code = http.StatusUnauthorized
		errorResponse.Message = "unauthorized"
		logger.Log(0, request.Header.Get("user"),
			fmt.Sprintf("%s: %v", errorResponse.Message, err))
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}

	var successResponse = models.SuccessResponse{
		Code:    http.StatusOK,
		Message: "W1R3: Host " + authRequest.ID + " Authorized",
		Response: models.SuccessfulLoginResponse{
			AuthToken: tokenString,
			ID:        authRequest.ID,
		},
	}
	successJSONResponse, jsonError := json.Marshal(successResponse)

	if jsonError != nil {
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logger.Log(0, request.Header.Get("user"),
			"error marshalling resp: ", err.Error())
		logic.ReturnErrorResponse(response, request, errorResponse)
		return
	}
	response.WriteHeader(http.StatusOK)
	response.Header().Set("Content-Type", "application/json")
	response.Write(successJSONResponse)
}

// swagger:route POST /api/hosts/keys host updateAllKeys
//
// Update keys for a network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: networkBodyResponse
func updateAllKeys(w http.ResponseWriter, r *http.Request) {
	var errorResponse = models.ErrorResponse{}
	w.Header().Set("Content-Type", "application/json")
	hosts, err := logic.GetAllHosts()
	if err != nil {
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logger.Log(0, r.Header.Get("user"),
			"error retrieving hosts ", err.Error())
		logic.ReturnErrorResponse(w, r, errorResponse)
		return
	}
	go func() {
		hostUpdate := models.HostUpdate{}
		hostUpdate.Action = models.UpdateKeys
		for _, host := range hosts {
			hostUpdate.Host = host
			logger.Log(2, "updating host", host.ID.String(), " for a key update")
			if err = mq.HostUpdate(&hostUpdate); err != nil {
				logger.Log(0, "failed to send update to node during a network wide key update", host.ID.String(), err.Error())
			}
		}
	}()
	logger.Log(2, r.Header.Get("user"), "updated keys for all hosts")
	w.WriteHeader(http.StatusOK)
}

// swagger:route POST /api/hosts/{hostid}keys host updateKeys
//
// Update keys for a network.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: networkBodyResponse
func updateKeys(w http.ResponseWriter, r *http.Request) {
	var errorResponse = models.ErrorResponse{}
	w.Header().Set("Content-Type", "application/json")
	var params = mux.Vars(r)
	hostid := params["hostid"]
	host, err := logic.GetHost(hostid)
	if err != nil {
		logger.Log(0, "failed to retrieve host", hostid, err.Error())
		errorResponse.Code = http.StatusBadRequest
		errorResponse.Message = err.Error()
		logger.Log(0, r.Header.Get("user"),
			"error retrieving hosts ", err.Error())
		logic.ReturnErrorResponse(w, r, errorResponse)
		return
	}
	go func() {
		hostUpdate := models.HostUpdate{
			Action: models.UpdateKeys,
			Host:   *host,
		}
		if err = mq.HostUpdate(&hostUpdate); err != nil {
			logger.Log(0, "failed to send host key update", host.ID.String(), err.Error())
		}
	}()
	logger.Log(2, r.Header.Get("user"), "updated key on host", host.Name)
	w.WriteHeader(http.StatusOK)
}
