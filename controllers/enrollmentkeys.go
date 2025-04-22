package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"github.com/gravitl/netmaker/auth"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/servercfg"
	"golang.org/x/exp/slog"
)

func enrollmentKeyHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/enrollment-keys", logic.SecurityCheck(true, http.HandlerFunc(createEnrollmentKey))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/enrollment-keys", logic.SecurityCheck(true, http.HandlerFunc(getEnrollmentKeys))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/enrollment-keys/{keyID}", logic.SecurityCheck(true, http.HandlerFunc(deleteEnrollmentKey))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/host/register/{token}", http.HandlerFunc(handleHostRegister)).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/enrollment-keys/{keyID}", logic.SecurityCheck(true, http.HandlerFunc(updateEnrollmentKey))).
		Methods(http.MethodPut)
}

// @Summary     Lists all EnrollmentKeys for admins
// @Router      /api/v1/enrollment-keys [get]
// @Tags        EnrollmentKeys
// @Security    oauth
// @Success     200 {array} models.EnrollmentKey
// @Failure     500 {object} models.ErrorResponse
func getEnrollmentKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := logic.GetAllEnrollmentKeys()
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to fetch enrollment keys: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	ret := []*models.EnrollmentKey{}
	for _, key := range keys {
		key := key
		if err = logic.Tokenize(&key, servercfg.GetAPIHost()); err != nil {
			logger.Log(0, r.Header.Get("user"), "failed to get token values for keys:", err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		ret = append(ret, &key)
	}
	// return JSON/API formatted keys
	logger.Log(2, r.Header.Get("user"), "fetched enrollment keys")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ret)
}

// @Summary     Deletes an EnrollmentKey from Netmaker server
// @Router      /api/v1/enrollment-keys/{keyid} [delete]
// @Tags        EnrollmentKeys
// @Security    oauth
// @Param       keyid path string true "Enrollment Key ID"
// @Success     200
// @Failure     500 {object} models.ErrorResponse
func deleteEnrollmentKey(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	keyID := params["keyID"]
	err := logic.DeleteEnrollmentKey(keyID, false)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to remove enrollment key: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logger.Log(2, r.Header.Get("user"), "deleted enrollment key", keyID)
	w.WriteHeader(http.StatusOK)
}

// @Summary     Creates an EnrollmentKey for hosts to register with server and join networks
// @Router      /api/v1/enrollment-keys [post]
// @Tags        EnrollmentKeys
// @Security    oauth
// @Param       body body models.APIEnrollmentKey true "Enrollment Key parameters"
// @Success     200 {object} models.EnrollmentKey
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
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
	v := validator.New()
	err = v.Struct(enrollmentKeyBody)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error validating request body: ",
			err.Error())
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(
				fmt.Errorf("validation error: name length must be between 3 and 32: %w", err),
				"badrequest",
			),
		)
		return
	}

	if existingKeys, err := logic.GetAllEnrollmentKeys(); err != nil {
		logger.Log(0, r.Header.Get("user"), "error validating request body: ",
			err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	} else {
		// check if any tags are duplicate
		existingTags := make(map[string]struct{})
		for _, existingKey := range existingKeys {
			for _, t := range existingKey.Tags {
				existingTags[t] = struct{}{}
			}
		}
		for _, t := range enrollmentKeyBody.Tags {
			if _, ok := existingTags[t]; ok {
				logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("key names must be unique"), "badrequest"))
				return
			}
		}
	}

	relayId := uuid.Nil
	if enrollmentKeyBody.Relay != "" {
		relayId, err = uuid.Parse(enrollmentKeyBody.Relay)
		if err != nil {
			logger.Log(0, r.Header.Get("user"), "error parsing relay id: ", err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	}

	newEnrollmentKey, err := logic.CreateEnrollmentKey(
		enrollmentKeyBody.UsesRemaining,
		newTime,
		enrollmentKeyBody.Networks,
		enrollmentKeyBody.Tags,
		enrollmentKeyBody.Groups,
		enrollmentKeyBody.Unlimited,
		relayId,
		false,
		enrollmentKeyBody.AutoEgress,
	)
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

// @Summary     Updates an EnrollmentKey. Updates are only limited to the relay to use
// @Router      /api/v1/enrollment-keys/{keyid} [put]
// @Tags        EnrollmentKeys
// @Security    oauth
// @Param       keyid path string true "Enrollment Key ID"
// @Param       body body models.APIEnrollmentKey true "Enrollment Key parameters"
// @Success     200 {object} models.EnrollmentKey
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func updateEnrollmentKey(w http.ResponseWriter, r *http.Request) {
	var enrollmentKeyBody models.APIEnrollmentKey
	params := mux.Vars(r)
	keyId := params["keyID"]

	err := json.NewDecoder(r.Body).Decode(&enrollmentKeyBody)
	if err != nil {
		slog.Error("error decoding request body", "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	relayId := uuid.Nil
	if enrollmentKeyBody.Relay != "" {
		relayId, err = uuid.Parse(enrollmentKeyBody.Relay)
		if err != nil {
			slog.Error("error parsing relay id", "error", err)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	}

	newEnrollmentKey, err := logic.UpdateEnrollmentKey(keyId, relayId, enrollmentKeyBody.Groups)
	if err != nil {
		slog.Error("failed to update enrollment key", "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	if err = logic.Tokenize(newEnrollmentKey, servercfg.GetAPIHost()); err != nil {
		slog.Error("failed to update enrollment key", "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	slog.Info("updated enrollment key", "id", keyId)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newEnrollmentKey)
}

// @Summary     Handles a Netclient registration with server and add nodes accordingly
// @Router      /api/v1/host/register/{token} [post]
// @Tags        EnrollmentKeys
// @Security    oauth
// @Param       token path string true "Enrollment Key Token"
// @Param       body body models.Host true "Host registration parameters"
// @Success     200 {object} models.RegisterResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func handleHostRegister(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
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
	// check if host already exists
	hostExists := false
	if hostExists = logic.HostExists(&newHost); hostExists && len(enrollmentKey.Networks) == 0 {
		logger.Log(
			0,
			"host",
			newHost.ID.String(),
			newHost.Name,
			"attempted to re-register with no networks",
		)
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(fmt.Errorf("host already exists"), "badrequest"),
		)
		return
	}
	// version check
	if !logic.IsVersionCompatible(newHost.Version) {
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
		logic.ReturnErrorResponse(
			w,
			r,
			logic.FormatError(fmt.Errorf("invalid enrollment key"), "badrequest"),
		)
		return
	}
	if !hostExists {
		newHost.PersistentKeepalive = models.DefaultPersistentKeepAlive
		// register host
		//logic.CheckHostPorts(&newHost)
		// create EMQX credentials and ACLs for host
		if servercfg.GetBrokerType() == servercfg.EmqxBrokerType {
			if err := mq.GetEmqxHandler().CreateEmqxUser(newHost.ID.String(), newHost.HostPass); err != nil {
				logger.Log(0, "failed to create host credentials for EMQX: ", err.Error())
				return
			}
		}

		if err = logic.CreateHost(&newHost); err != nil {
			logger.Log(
				0,
				"host",
				newHost.ID.String(),
				newHost.Name,
				"failed registration -",
				err.Error(),
			)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	} else {
		// need to revise the list of networks from key
		// based on the ones host currently has
		networksToAdd := []string{}
		currentNets := logic.GetHostNetworks(newHost.ID.String())
		for _, newNet := range enrollmentKey.Networks {
			if !logic.StringSliceContains(currentNets, newNet) {
				networksToAdd = append(networksToAdd, newNet)
			}
		}
		enrollmentKey.Networks = networksToAdd
		currHost, err := logic.GetHost(newHost.ID.String())
		if err != nil {
			slog.Error("failed registration", "hostID", newHost.ID.String(), "hostName", newHost.Name, "error", err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		logic.UpdateHostFromClient(&newHost, currHost)
		err = logic.UpsertHost(currHost)
		if err != nil {
			slog.Error("failed to update host", "id", currHost.ID, "error", err)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	}
	// ready the response
	server := logic.GetServerInfo()
	server.TrafficKey = key
	response := models.RegisterResponse{
		ServerConf:    server,
		RequestedHost: newHost,
	}
	logger.Log(0, newHost.Name, newHost.ID.String(), "registered with Netmaker")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&response)
	// notify host of changes, peer and node updates
	go auth.CheckNetRegAndHostUpdate(enrollmentKey.Networks, &newHost, enrollmentKey.Relay, enrollmentKey.Groups)
}
