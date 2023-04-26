package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/auth"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
)

func enrollmentKeyHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/enrollment-keys", logic.SecurityCheck(true, http.HandlerFunc(createEnrollmentKey))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/enrollment-keys", logic.SecurityCheck(true, http.HandlerFunc(getEnrollmentKeys))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/enrollment-keys/{keyID}", logic.SecurityCheck(true, http.HandlerFunc(deleteEnrollmentKey))).Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/host/register/{token}", http.HandlerFunc(handleHostRegister)).Methods(http.MethodPost)
}

// swagger:route GET /api/v1/enrollment-keys enrollmentKeys getEnrollmentKeys
//
// Lists all EnrollmentKeys for admins.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: getEnrollmentKeysSlice
func getEnrollmentKeys(w http.ResponseWriter, r *http.Request) {
	currentKeys, err := logic.GetAllEnrollmentKeys()
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to fetch enrollment keys: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	for i := range currentKeys {
		currentKey := currentKeys[i]
		if err = logic.Tokenize(currentKey, servercfg.GetAPIHost()); err != nil {
			logger.Log(0, r.Header.Get("user"), "failed to get token values for keys:", err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	}
	// return JSON/API formatted keys
	logger.Log(2, r.Header.Get("user"), "fetched enrollment keys")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(currentKeys)
}

// swagger:route DELETE /api/v1/enrollment-keys/{keyID} enrollmentKeys deleteEnrollmentKey
//
// Deletes an EnrollmentKey from Netmaker server.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: deleteEnrollmentKeyResponse
func deleteEnrollmentKey(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	keyID := params["keyID"]
	err := logic.DeleteEnrollmentKey(keyID)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to remove enrollment key: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(2, r.Header.Get("user"), "deleted enrollment key", keyID)
	w.WriteHeader(http.StatusOK)
}

// swagger:route POST /api/v1/enrollment-keys enrollmentKeys createEnrollmentKey
//
// Creates an EnrollmentKey for hosts to use on Netmaker server.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: createEnrollmentKeyResponse
func createEnrollmentKey(w http.ResponseWriter, r *http.Request) {

	var enrollmentKeyBody models.APIEnrollmentKey

	err := json.NewDecoder(r.Body).Decode(&enrollmentKeyBody)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	var newTime time.Time
	if enrollmentKeyBody.Expiration > 0 {
		newTime = time.Unix(enrollmentKeyBody.Expiration, 0)
	}

	newEnrollmentKey, err := logic.CreateEnrollmentKey(enrollmentKeyBody.UsesRemaining, newTime, enrollmentKeyBody.Networks, enrollmentKeyBody.Tags, enrollmentKeyBody.Unlimited)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to create enrollment key:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	if err = logic.Tokenize(newEnrollmentKey, servercfg.GetAPIHost()); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to create enrollment key:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(2, r.Header.Get("user"), "created enrollment key")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newEnrollmentKey)
}

// swagger:route POST /api/v1/enrollment-keys/{token} enrollmentKeys handleHostRegister
//
// Handles a Netclient registration with server and add nodes accordingly.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: handleHostRegisterResponse
func handleHostRegister(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	token := params["token"]
	logger.Log(0, "received registration attempt with token", token)
	// check if token exists
	enrollmentKey, err := logic.DeTokenize(token)
	if err != nil {
		logger.Log(0, "invalid enrollment key used", token, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	// get the host
	var newHost models.Host
	if err = json.NewDecoder(r.Body).Decode(&newHost); err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	hostExists := false
	// re-register host with turn just in case.
	err = logic.RegisterHostWithTurn(newHost.ID.String(), newHost.HostPass)
	if err != nil {
		logger.Log(0, "failed to register host with turn server: ", err.Error())
	}
	// check if host already exists
	if hostExists = logic.HostExists(&newHost); hostExists && len(enrollmentKey.Networks) == 0 {
		logger.Log(0, "host", newHost.ID.String(), newHost.Name, "attempted to re-register with no networks")
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("host already exists"), "badrequest"))
		return
	}
	// version check
	if !logic.IsVersionComptatible(newHost.Version) {
		err := fmt.Errorf("bad client version on register: %s", newHost.Version)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if newHost.TrafficKeyPublic == nil && newHost.OS != models.OS_Types.IoT {
		err := fmt.Errorf("missing traffic key")
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	key, keyErr := logic.RetrievePublicTrafficKey()
	if keyErr != nil {
		logger.Log(0, "error retrieving key:", keyErr.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	// use the token
	if ok := logic.TryToUseEnrollmentKey(enrollmentKey); !ok {
		logger.Log(0, "host", newHost.ID.String(), newHost.Name, "failed registration")
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("invalid enrollment key"), "badrequest"))
		return
	}
	hostPass := newHost.HostPass
	if !hostExists {
		// register host
		logic.CheckHostPorts(&newHost)
		// create EMQX credentials and ACLs for host
		if servercfg.GetBrokerType() == servercfg.EmqxBrokerType {
			if err := mq.CreateEmqxUser(newHost.ID.String(), newHost.HostPass, false); err != nil {
				logger.Log(0, "failed to create host credentials for EMQX: ", err.Error())
				return
			}
			if err := mq.CreateHostACL(newHost.ID.String(), servercfg.GetServerInfo().Server); err != nil {
				logger.Log(0, "failed to add host ACL rules to EMQX: ", err.Error())
				return
			}
		}
		if err = logic.CreateHost(&newHost); err != nil {
			logger.Log(0, "host", newHost.ID.String(), newHost.Name, "failed registration -", err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	} else {
		// need to revise the list of networks from key
		// based on the ones host currently has
		var networksToAdd = []string{}
		currentNets := logic.GetHostNetworks(newHost.ID.String())
		for _, newNet := range enrollmentKey.Networks {
			if !logic.StringSliceContains(currentNets, newNet) {
				networksToAdd = append(networksToAdd, newNet)
			}
		}
		enrollmentKey.Networks = networksToAdd
	}
	// ready the response
	server := servercfg.GetServerInfo()
	server.TrafficKey = key
	if servercfg.GetBrokerType() == servercfg.EmqxBrokerType {
		// set MQ username and password for EMQX clients
		server.MQUserName = newHost.ID.String()
		server.MQPassword = hostPass
	}
	response := models.RegisterResponse{
		ServerConf:    server,
		RequestedHost: newHost,
	}
	logger.Log(0, newHost.Name, newHost.ID.String(), "registered with Netmaker")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&response)
	// notify host of changes, peer and node updates
	go auth.CheckNetRegAndHostUpdate(enrollmentKey.Networks, &newHost)
}
