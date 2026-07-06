package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/schema"
	"golang.org/x/exp/slog"

	"github.com/gravitl/netmaker/auth"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/middleware"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	"github.com/gravitl/netmaker/scope"
	"github.com/gravitl/netmaker/servercfg"
)

func enrollmentKeyHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/enrollment-keys", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(createEnrollmentKey)))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/enrollment-keys", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(getEnrollmentKeys)))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v2/enrollment-keys", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(listEnrollmentKeys)))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/enrollment-keys/network/{network}/default", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(getDefaultEnrollmentKeyForNetwork)))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/enrollment-keys/{keyID}/regenerate-token", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(regenerateEnrollmentKeyToken)))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/enrollment-keys/{keyID}", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(deleteEnrollmentKey)))).
		Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/host/register/{token}", middleware.Scope(scope.TenantScope, http.HandlerFunc(handleHostRegister))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/enrollment-keys/{keyID}", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(updateEnrollmentKey)))).
		Methods(http.MethodPut)
}

// @Summary     Lists all EnrollmentKeys
// @Router      /api/v1/enrollment-keys [get]
// @Tags        EnrollmentKeys
// @Security    oauth
// @Produce     json
// @Success     200 {array} schema.EnrollmentKey
// @Failure     500 {object} models.ErrorResponse
func getEnrollmentKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := (&schema.EnrollmentKey{}).ListAll(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to fetch enrollment keys:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	for i := range keys {
		if err = logic.Tokenize(r.Context(), &keys[i], servercfg.GetAPIHost()); err != nil {
			logger.Log(0, r.Header.Get("user"), "failed to tokenize enrollment key:", err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	}

	resp := make([]models.EnrollmentKey, 0, len(keys))
	for _, key := range keys {
		networks := make([]string, 0)
		networks = append(networks, key.Networks...)

		keyType := models.KeyType(key.Type)

		var relay uuid.UUID
		if key.GatewayID != nil {
			relay, _ = uuid.Parse(*key.GatewayID)
		}

		var groups []models.TagID
		for _, tag := range key.Tags {
			groups = append(groups, models.TagID(tag))
		}
		resp = append(resp, models.EnrollmentKey{
			Expiration:        key.Expiration,
			UsesRemaining:     key.UsesRemaining,
			Value:             key.Value,
			Networks:          networks,
			Unlimited:         key.Unlimited,
			Tags:              []string{key.Name},
			Token:             key.Token,
			Type:              keyType,
			Relay:             relay,
			Groups:            groups,
			Default:           key.Default,
			AutoEgress:        key.AutoEgress,
			AutoAssignGateway: key.AutoAssignGateway,
		})
	}

	logger.Log(2, r.Header.Get("user"), "fetched enrollment keys")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// @Summary     Lists EnrollmentKeys (paginated)
// @Router      /api/v2/enrollment-keys [get]
// @Tags        EnrollmentKeys
// @Security    oauth
// @Produce     json
// @Param       page query int false "Page number (default 1)"
// @Param       per_page query int false "Items per page (default 10, max 100)"
// @Param       q query string false "Search across name, networks and tags"
// @Success     200 {object} models.PaginatedResponse
// @Failure     500 {object} models.ErrorResponse
func listEnrollmentKeys(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	q := r.URL.Query().Get("q")

	var filters, queryOptions []dbtypes.Option
	if q != "" {
		filters = append(filters, dbtypes.WithSearchQuery(q, "name", "networks", "tags"))
	}
	queryOptions = append(queryOptions, filters...)
	queryOptions = append(queryOptions, dbtypes.InAscOrder("created_at"))
	queryOptions = append(queryOptions, dbtypes.WithPagination(page, pageSize))

	keys, err := (&schema.EnrollmentKey{}).ListAll(r.Context(), queryOptions...)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to fetch enrollment keys:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	total, err := (&schema.EnrollmentKey{}).Count(r.Context(), filters...)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to fetch enrollment keys:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	for i := range keys {
		if err = logic.Tokenize(r.Context(), &keys[i], servercfg.GetAPIHost()); err != nil {
			logger.Log(0, r.Header.Get("user"), "failed to tokenize enrollment key:", err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	logger.Log(2, r.Header.Get("user"), "fetched enrollment keys")
	logic.ReturnSuccessResponseWithJson(w, r, models.PaginatedResponse{
		Data:       keys,
		Page:       page,
		PerPage:    pageSize,
		Total:      total,
		TotalPages: totalPages,
	}, "fetched enrollment keys")
}

// @Summary     Get the default enrollment key for a network
// @Router      /api/v1/enrollment-keys/network/{network}/default [get]
// @Tags        EnrollmentKeys
// @Security    oauth
// @Param       network path string true "Network name"
// @Produce     json
// @Success     200 {object} schema.EnrollmentKey
// @Failure     404 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func getDefaultEnrollmentKeyForNetwork(w http.ResponseWriter, r *http.Request) {
	network := mux.Vars(r)["network"]
	if network == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network is required"), "badrequest"))
		return
	}

	net := &schema.Network{Name: network}
	if err := net.Get(r.Context()); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("network not found"), "badrequest"))
		return
	}

	key, err := logic.GetDefaultEnrollmentKeyForNetwork(r.Context(), network)
	if err != nil {
		if errors.Is(err, logic.EnrollmentErrors.NoKeyFound) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
		logger.Log(0, r.Header.Get("user"), "failed to fetch default enrollment key for network", network, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	if err = logic.Tokenize(r.Context(), key, servercfg.GetAPIHost()); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to tokenize default enrollment key:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logger.Log(2, r.Header.Get("user"), "fetched default enrollment key for network", network)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(key)
}

// @Summary     Regenerate an enrollment key token
// @Description Replaces the enrollment key value and invalidates any previously issued registration tokens.
// @Router      /api/v1/enrollment-keys/{keyID}/regenerate-token [post]
// @Tags        EnrollmentKeys
// @Security    oauth
// @Param       keyID path string true "Enrollment Key value"
// @Produce     json
// @Success     200 {object} schema.EnrollmentKey
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func regenerateEnrollmentKeyToken(w http.ResponseWriter, r *http.Request) {
	keyID := mux.Vars(r)["keyID"]
	if keyID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("key id is required"), "badrequest"))
		return
	}

	currKey, err := logic.GetEnrollmentKey(r.Context(), keyID)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	newKey, err := logic.RegenerateEnrollmentKeyToken(r.Context(), keyID)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to regenerate enrollment key token:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	if err = logic.Tokenize(r.Context(), newKey, servercfg.GetAPIHost()); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to tokenize regenerated enrollment key:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logic.LogEvent(&models.Event{
		Action: schema.Update,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   newKey.Value,
			Name: enrollmentKeyName(newKey),
			Type: schema.EnrollmentKeySub,
		},
		Diff: models.Diff{
			Old: currKey,
			New: newKey,
		},
		Origin: schema.Dashboard,
	})

	logger.Log(2, r.Header.Get("user"), "regenerated enrollment key token", keyID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newKey)
}

// @Summary     Deletes an EnrollmentKey from Netmaker server
// @Router      /api/v1/enrollment-keys/{keyID} [delete]
// @Tags        EnrollmentKeys
// @Security    oauth
// @Param       keyID path string true "Enrollment Key value"
// @Success     200 {string} string
// @Failure     500 {object} models.ErrorResponse
func deleteEnrollmentKey(w http.ResponseWriter, r *http.Request) {
	keyID := mux.Vars(r)["keyID"]
	key, err := logic.GetEnrollmentKey(r.Context(), keyID)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if err = logic.DeleteEnrollmentKey(r.Context(), keyID, false); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to remove enrollment key: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.LogEvent(&models.Event{
		Action: schema.Delete,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   keyID,
			Name: enrollmentKeyName(key),
			Type: schema.EnrollmentKeySub,
		},
		Origin: schema.Dashboard,
		Diff: models.Diff{
			Old: key,
			New: nil,
		},
	})
	logger.Log(2, r.Header.Get("user"), "deleted enrollment key", keyID)
	w.WriteHeader(http.StatusOK)
}

// @Summary     Creates an EnrollmentKey for hosts to register with server and join networks
// @Router      /api/v1/enrollment-keys [post]
// @Tags        EnrollmentKeys
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       body body models.APIEnrollmentKey true "Enrollment Key parameters"
// @Success     200 {object} schema.EnrollmentKey
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func createEnrollmentKey(w http.ResponseWriter, r *http.Request) {
	var req models.APIEnrollmentKey
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	var newTime time.Time
	if req.Expiration > 0 {
		newTime = time.Unix(req.Expiration, 0)
	}

	v := validator.New()
	if err := v.Struct(req); err != nil {
		logger.Log(0, r.Header.Get("user"), "error validating request body:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(
			fmt.Errorf("validation error: name length must be between 3 and 32: %w", err), "badrequest"))
		return
	}

	existingKeys, err := logic.GetAllEnrollmentKeys(r.Context())
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error fetching enrollment keys:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	// check if any network names are duplicate across existing keys
	existingNetworks := make(map[string]struct{})
	for _, existingKey := range existingKeys {
		for _, n := range existingKey.Networks {
			existingNetworks[n] = struct{}{}
		}
	}
	for _, t := range req.Tags {
		if _, ok := existingNetworks[t]; ok {
			logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("key names must be unique"), "badrequest"))
			return
		}
	}

	if req.Default && len(req.Networks) == 0 {
		logic.ReturnErrorResponse(w, r, logic.FormatError(
			errors.New("default enrollment keys require at least one network or tag"), "badrequest"))
		return
	}

	relayId := uuid.Nil
	if req.Relay != "" {
		relayId, err = uuid.Parse(req.Relay)
		if err != nil {
			logger.Log(0, r.Header.Get("user"), "error parsing relay id:", err.Error())
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	}

	newKey, err := logic.CreateEnrollmentKey(
		r.Context(),
		req.UsesRemaining,
		newTime,
		req.Networks,
		req.Tags,
		req.Groups,
		req.Unlimited,
		relayId,
		req.Default,
		req.AutoEgress,
		req.AutoAssignGateway,
	)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to create enrollment key:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	if err = logic.Tokenize(r.Context(), newKey, servercfg.GetAPIHost()); err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to tokenize enrollment key:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logic.LogEvent(&models.Event{
		Action: schema.Create,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   newKey.Value,
			Name: enrollmentKeyName(newKey),
			Type: schema.EnrollmentKeySub,
		},
		Origin: schema.Dashboard,
	})
	logger.Log(2, r.Header.Get("user"), "created enrollment key")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newKey)
}

// @Summary     Updates an EnrollmentKey
// @Router      /api/v1/enrollment-keys/{keyID} [put]
// @Tags        EnrollmentKeys
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       keyID path string true "Enrollment Key value"
// @Param       body body models.APIEnrollmentKey true "Enrollment Key parameters"
// @Success     200 {object} schema.EnrollmentKey
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func updateEnrollmentKey(w http.ResponseWriter, r *http.Request) {
	var req models.APIEnrollmentKey
	keyId := mux.Vars(r)["keyID"]

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.Error("error decoding request body", "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	if req.Relay != "" {
		if _, err := uuid.Parse(req.Relay); err != nil {
			slog.Error("error parsing relay id", "error", err)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}
	}

	currKey, _ := logic.GetEnrollmentKey(r.Context(), keyId)

	newKey, err := logic.UpdateEnrollmentKey(r.Context(), keyId, &req)
	if err != nil {
		slog.Error("failed to update enrollment key", "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	if err = logic.Tokenize(r.Context(), newKey, servercfg.GetAPIHost()); err != nil {
		slog.Error("failed to tokenize enrollment key", "error", err)
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	logic.LogEvent(&models.Event{
		Action: schema.Update,
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		TriggeredBy: r.Header.Get("user"),
		Target: models.Subject{
			ID:   newKey.Value,
			Name: enrollmentKeyName(newKey),
			Type: schema.EnrollmentKeySub,
		},
		Diff: models.Diff{
			Old: currKey,
			New: newKey,
		},
		Origin: schema.Dashboard,
	})
	slog.Info("updated enrollment key", "id", keyId)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(newKey)
}

// @Summary     Handles a Netclient registration with server and add nodes accordingly
// @Router      /api/v1/host/register/{token} [post]
// @Tags        EnrollmentKeys
// @Accept      json
// @Produce     json
// @Param       token path string true "Enrollment Key Token"
// @Param       body body schema.Host true "Host registration parameters"
// @Success     200 {object} models.RegisterResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func handleHostRegister(w http.ResponseWriter, r *http.Request) {
	token := mux.Vars(r)["token"]
	logger.Log(0, "received registration attempt with token", token)

	enrollmentKey, err := logic.DeTokenize(r.Context(), token)
	if err != nil {
		logger.Log(0, "invalid enrollment key used", token, err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}

	var newHost schema.Host
	if err = json.NewDecoder(r.Body).Decode(&newHost); err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}

	hostExists := false
	if hostExists = logic.HostExists(&newHost); hostExists && len(enrollmentKey.Networks) == 0 {
		logger.Log(0, "host", newHost.ID.String(), newHost.Name, "attempted to re-register with no networks")
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("host already exists"), "badrequest"))
		return
	}

	if !logic.IsVersionCompatible(newHost.Version) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(
			fmt.Errorf("bad client version on register: %s", newHost.Version), "badrequest"))
		return
	}
	if newHost.TrafficKeyPublic == nil && newHost.OS != models.OS_Types.IoT {
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("missing traffic key"), "badrequest"))
		return
	}

	trafficKey, keyErr := logic.RetrievePublicTrafficKey()
	if keyErr != nil {
		logger.Log(0, "error retrieving key:", keyErr.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(keyErr, "internal"))
		return
	}

	if ok := logic.TryToUseEnrollmentKey(r.Context(), enrollmentKey); !ok {
		logger.Log(0, "host", newHost.ID.String(), newHost.Name, "failed registration")
		logic.ReturnErrorResponse(w, r, logic.FormatError(fmt.Errorf("invalid enrollment key"), "badrequest"))
		return
	}

	keyTags := make(map[models.TagID]struct{})
	for _, tagI := range enrollmentKey.Tags {
		keyTags[models.TagID(tagI)] = struct{}{}
	}

	var joinNetworks []string
	for _, netI := range enrollmentKey.Networks {
		violations, _ := logic.CheckPostureViolationsForHost(&newHost, keyTags, schema.NetworkID(netI), true)
		if len(violations) == 0 {
			joinNetworks = append(joinNetworks, netI)
		}
	}
	if len(joinNetworks) != len(enrollmentKey.Networks) && len(joinNetworks) == 0 {
		logic.ReturnErrorResponse(w, r,
			logic.FormatError(errors.New("access blocked: this device doesn't meet security requirements"), logic.Forbidden))
		return
	}

	// copy key so network edits don't mutate the stored key
	key := *enrollmentKey
	key.Networks = joinNetworks

	var host *schema.Host
	if !hostExists {
		newHost.PersistentKeepalive = models.DefaultPersistentKeepAlive
		_ = logic.CheckHostPorts(&newHost)
		if servercfg.GetBrokerType() == servercfg.EmqxBrokerType {
			if err := mq.GetEmqxHandler().CreateEmqxUser(newHost.ID.String(), newHost.HostPass); err != nil {
				logger.Log(0, "failed to create host credentials for EMQX: ", err.Error())
				return
			}
		}
		if err = logic.CreateHost(&newHost); err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		host = &newHost
	} else {
		currHost := &schema.Host{ID: newHost.ID}
		if err = currHost.Get(r.Context()); err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		endpointChanged, _ := logic.UpdateHostFromClient(&newHost, currHost)
		if endpointChanged {
			logic.CheckHostPorts(currHost)
		}
		if err = logic.UpsertHost(currHost); err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
			return
		}
		host = currHost
	}

	server := logic.GetServerInfo()
	server.TrafficKey = trafficKey
	response := models.RegisterResponse{
		ServerConf:    server,
		RequestedHost: *host,
	}

	logger.Log(0, host.Name, host.ID.String(), "registered with Netmaker")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&response)
	go auth.CheckNetRegAndHostUpdate(key, host, r.Header.Get("user"))
}

// enrollmentKeyName returns a human-readable label for audit events.
func enrollmentKeyName(key *schema.EnrollmentKey) string {
	if key != nil && key.Name != "" {
		return key.Name
	}
	if key != nil {
		return key.Value
	}
	return ""
}
