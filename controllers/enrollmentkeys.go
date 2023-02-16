package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
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
// Lists all hosts.
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
		if err = logic.Tokenize(currentKey, servercfg.GetServer()); err != nil {
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
// Deletes a Netclient host from Netmaker server.
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

	if err = logic.Tokenize(newEnrollmentKey, servercfg.GetServer()); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to create enrollment key:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(2, r.Header.Get("user"), "created enrollment key")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newEnrollmentKey)
}

// swagger:route POST /api/v1/enrollment-keys/{token} enrollmentKeys deleteEnrollmentKey
//
// Deletes a Netclient host from Netmaker server.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: hostRegisterResponse
func handleHostRegister(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	token := params["token"]
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
	// check if host already exists
	if ok := logic.HostExists(&newHost); ok {
		logger.Log(0, "host", newHost.ID.String(), newHost.Name, "attempted to re-register")
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("host already exists"), "badrequest"))
		return
	}
	// version check
	if !logic.IsVersionComptatible(newHost.Version) || newHost.TrafficKeyPublic == nil {
		err := fmt.Errorf("incompatible netclient")
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
	// register host
	logic.CheckHostPorts(&newHost)
	if err = logic.CreateHost(&newHost); err != nil {
		logger.Log(0, "host", newHost.ID.String(), newHost.Name, "failed registration -", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	// ready the response
	server := servercfg.GetServerInfo()
	server.TrafficKey = key
	logger.Log(2, r.Header.Get("user"), "deleted enrollment key", token)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&server)
	// notify host of changes, peer and node updates
	go checkNetRegAndHostUpdate(enrollmentKey.Networks, &newHost)
}

// run through networks and send a host update
func checkNetRegAndHostUpdate(networks []string, h *models.Host) {
	// publish host update through MQ
	if servercfg.IsMessageQueueBackend() {
		if err := mq.HostUpdate(&models.HostUpdate{
			Action: models.UpdateHost,
			Host:   *h,
		}); err != nil {
			logger.Log(0, "failed to send host update after registration:", h.ID.String(), err.Error())
		}
	}

	for i := range networks {
		if ok, _ := logic.NetworkExists(networks[i]); ok {
			newNode, err := logic.UpdateHostNetwork(h, networks[i], true)
			if err != nil {
				logger.Log(0, "failed to add host to network:", h.ID.String(), h.Name, networks[i], err.Error())
				continue
			}
			logger.Log(1, "added new node", newNode.ID.String(), "to host", h.Name)
			if servercfg.IsMessageQueueBackend() {
				if err = mq.HostUpdate(&models.HostUpdate{
					Action: models.JoinHostToNetwork,
					Host:   *h,
					Node:   *newNode,
				}); err != nil {
					logger.Log(0, "failed to send host update to", h.ID.String(), networks[i], err.Error())
				}
			}
		}
	}

	if servercfg.IsMessageQueueBackend() {
		if err := mq.PublishPeerUpdate(); err != nil {
			logger.Log(0, "failed to publish peer update after host registration -", err.Error())
		}
	}
}
