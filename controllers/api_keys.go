package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/middleware"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
)

func apiKeyHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/api-keys", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(listAPIKeys)))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/api-keys", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(createAPIKey)))).
		Methods(http.MethodPost)
	r.HandleFunc("/api/v1/api-keys/{id}", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(getAPIKey)))).
		Methods(http.MethodGet)
	r.HandleFunc("/api/v1/api-keys/{id}", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(renameAPIKey)))).
		Methods(http.MethodPatch)
	r.HandleFunc("/api/v1/api-keys/{id}", middleware.Scope(scope.TenantScope, logic.SecurityCheck(true, http.HandlerFunc(revokeAPIKey)))).
		Methods(http.MethodDelete)
}

// requireAPIKeyAdmin ensures the caller is an Admin/SuperAdmin user JWT (not an API key).
func requireAPIKeyAdmin(w http.ResponseWriter, r *http.Request) (*schema.User, bool) {
	if logic.IsAPIKeyAuth(r.Context()) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("api keys cannot manage api keys"), logic.Forbidden))
		return nil, false
	}
	username := r.Header.Get("user")
	if username == logic.MasterUser {
		return &schema.User{Username: logic.MasterUser, PlatformRoleID: schema.SuperAdminRole}, true
	}
	user := &schema.User{Username: username}
	if err := user.GetWithMembership(r.Context()); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.UnAuthorized))
		return nil, false
	}
	if user.PlatformRoleID != schema.SuperAdminRole && user.PlatformRoleID != schema.AdminRole {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("only platform admins can manage api keys"), logic.Forbidden))
		return nil, false
	}
	return user, true
}

// @Summary     List API keys
// @Router      /api/v1/api-keys [get]
// @Tags        APIKeys
// @Security    oauth
// @Produce     json
// @Success     200 {array} models.APIKeyResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     403 {object} models.ErrorResponse
func listAPIKeys(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAPIKeyAdmin(w, r); !ok {
		return
	}
	keys, err := logic.ListAPIKeys(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}
	resp := make([]models.APIKeyResponse, 0, len(keys))
	for i := range keys {
		resp = append(resp, logic.ToAPIKeyResponse(&keys[i]))
	}
	logic.ReturnSuccessResponseWithJson(w, r, resp, "fetched api keys")
}

// @Summary     Create an API key
// @Router      /api/v1/api-keys [post]
// @Tags        APIKeys
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       body body models.CreateAPIKeyRequest true "API key request"
// @Success     200 {object} models.CreateAPIKeyResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     403 {object} models.ErrorResponse
func createAPIKey(w http.ResponseWriter, r *http.Request) {
	caller, ok := requireAPIKeyAdmin(w, r)
	if !ok {
		return
	}
	var req models.CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Log(0, "error decoding api key create body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}
	key, secret, err := logic.CreateAPIKey(r.Context(), &req, caller.Username)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	safe := logic.ToAPIKeyResponse(key)
	logic.LogEvent(r.Context(), &models.Event{
		Action: schema.Create,
		Source: models.Subject{
			ID:   caller.Username,
			Name: caller.Username,
			Type: schema.UserSub,
		},
		TriggeredBy: caller.Username,
		Target: models.Subject{
			ID:   key.ID,
			Name: key.Name,
			Type: schema.APIKeySub,
			Info: safe,
		},
		Origin: schema.Dashboard,
	})

	logic.ReturnSuccessResponseWithJson(w, r, models.CreateAPIKeyResponse{
		APIKeyResponse: safe,
		Key:            secret,
	}, "api key created")
}

// @Summary     Get an API key
// @Router      /api/v1/api-keys/{id} [get]
// @Tags        APIKeys
// @Security    oauth
// @Produce     json
// @Param       id path string true "API key ID"
// @Success     200 {object} models.APIKeyResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     403 {object} models.ErrorResponse
// @Failure     404 {object} models.ErrorResponse
func getAPIKey(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAPIKeyAdmin(w, r); !ok {
		return
	}
	id := mux.Vars(r)["id"]
	key, err := logic.GetAPIKey(r.Context(), id)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.NotFound))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, logic.ToAPIKeyResponse(key), "fetched api key")
}

// @Summary     Rename an API key
// @Router      /api/v1/api-keys/{id} [patch]
// @Tags        APIKeys
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       id path string true "API key ID"
// @Param       body body models.RenameAPIKeyRequest true "Rename request"
// @Success     200 {object} models.APIKeyResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     403 {object} models.ErrorResponse
// @Failure     404 {object} models.ErrorResponse
func renameAPIKey(w http.ResponseWriter, r *http.Request) {
	caller, ok := requireAPIKeyAdmin(w, r)
	if !ok {
		return
	}
	id := mux.Vars(r)["id"]
	var req models.RenameAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}
	key, err := logic.RenameAPIKey(r.Context(), id, req.Name)
	if err != nil {
		errType := logic.BadReq
		if err.Error() == "cannot rename a revoked api key" {
			errType = logic.Forbidden
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, errType))
		return
	}
	safe := logic.ToAPIKeyResponse(key)
	logic.LogEvent(r.Context(), &models.Event{
		Action: schema.Update,
		Source: models.Subject{
			ID:   caller.Username,
			Name: caller.Username,
			Type: schema.UserSub,
		},
		TriggeredBy: caller.Username,
		Target: models.Subject{
			ID:   key.ID,
			Name: key.Name,
			Type: schema.APIKeySub,
			Info: safe,
		},
		Origin: schema.Dashboard,
	})
	logic.ReturnSuccessResponseWithJson(w, r, safe, "api key renamed")
}

// @Summary     Revoke an API key
// @Router      /api/v1/api-keys/{id} [delete]
// @Tags        APIKeys
// @Security    oauth
// @Produce     json
// @Param       id path string true "API key ID"
// @Success     200 {object} models.SuccessResponse
// @Failure     401 {object} models.ErrorResponse
// @Failure     403 {object} models.ErrorResponse
// @Failure     404 {object} models.ErrorResponse
func revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	caller, ok := requireAPIKeyAdmin(w, r)
	if !ok {
		return
	}
	id := mux.Vars(r)["id"]
	key, err := logic.RevokeAPIKey(r.Context(), id)
	if err != nil {
		errType := logic.BadReq
		if err.Error() == "api key already revoked" {
			errType = logic.Forbidden
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, errType))
		return
	}
	safe := logic.ToAPIKeyResponse(key)
	logic.LogEvent(r.Context(), &models.Event{
		Action: schema.Delete,
		Source: models.Subject{
			ID:   caller.Username,
			Name: caller.Username,
			Type: schema.UserSub,
		},
		TriggeredBy: caller.Username,
		Target: models.Subject{
			ID:   key.ID,
			Name: key.Name,
			Type: schema.APIKeySub,
			Info: safe,
		},
		Origin: schema.Dashboard,
	})
	logic.ReturnSuccessResponse(w, r, "api key revoked")
}
