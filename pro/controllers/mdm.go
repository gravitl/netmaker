package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/pro/mdm"
	"github.com/gravitl/netmaker/schema"
)

// MDMHandlers registers the MDM integrations endpoints. Provider credentials
// themselves are persisted via the existing ServerSettings endpoints; these
// routes only cover actions that don't fit the settings CRUD.
func MDMHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/mdm/provider_types",
		logic.SecurityCheck(true, http.HandlerFunc(listMDMProviderTypes))).
		Methods(http.MethodGet)

	r.HandleFunc("/api/v1/mdm/verify",
		logic.SecurityCheck(true, http.HandlerFunc(verifyMDM))).
		Methods(http.MethodPost)

	r.HandleFunc("/api/v1/mdm/sync",
		logic.SecurityCheck(true, http.HandlerFunc(triggerMDMSync))).
		Methods(http.MethodPost)

	r.HandleFunc("/api/v1/mdm/device_state",
		logic.SecurityCheck(true, http.HandlerFunc(listMDMDeviceState))).
		Methods(http.MethodGet)
}

// @Summary     List built-in MDM provider types
// @Router      /api/v1/mdm/provider_types [get]
// @Tags        MDM
// @Security    oauth
// @Produce     json
// @Success     200 {array}  mdm.ProviderType
func listMDMProviderTypes(w http.ResponseWriter, r *http.Request) {
	logic.ReturnSuccessResponseWithJson(w, r, mdm.ListProviderTypes(), "fetched mdm provider types")
}

// @Summary     Verify the active (or supplied) MDM credentials
// @Router      /api/v1/mdm/verify [post]
// @Tags        MDM
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       body body models.ServerSettings false "Optional draft settings; when omitted, the saved settings are used"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func verifyMDM(w http.ResponseWriter, r *http.Request) {
	s := logic.GetServerSettings()
	if r.ContentLength > 0 {
		var draft models.ServerSettings
		if err := json.NewDecoder(r.Body).Decode(&draft); err == nil {
			// Allow a "test before save" flow: any provider-specific masked
			// values fall back to the saved settings.
			if draft.MDMClientSecret == logic.Mask() {
				draft.MDMClientSecret = s.MDMClientSecret
			}
			if draft.MDMProvider != models.MDMProviderDisabled {
				s = draft
			}
		}
	}
	if s.MDMProvider == models.MDMProviderDisabled {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("no MDM provider configured"), "badrequest"))
		return
	}
	p, err := mdm.BuildActive(s)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	if p == nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("no MDM provider configured"), "badrequest"))
		return
	}
	if err := p.Verify(r.Context()); err != nil {
		logic.LogEvent(&models.Event{
			Action:      schema.MDMVerify,
			TriggeredBy: r.Header.Get("user"),
			Source: models.Subject{
				ID:   r.Header.Get("user"),
				Name: r.Header.Get("user"),
				Type: schema.UserSub,
			},
			Target: models.Subject{
				ID:   string(s.MDMProvider),
				Name: string(s.MDMProvider),
				Type: schema.MDMSub,
			},
			Origin: schema.Dashboard,
			Diff: models.Diff{
				New: map[string]interface{}{"status": "failed", "error": err.Error()},
			},
		})
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	logic.LogEvent(&models.Event{
		Action:      schema.MDMVerify,
		TriggeredBy: r.Header.Get("user"),
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		Target: models.Subject{
			ID:   string(s.MDMProvider),
			Name: string(s.MDMProvider),
			Type: schema.MDMSub,
		},
		Origin: schema.Dashboard,
		Diff: models.Diff{
			New: map[string]interface{}{"status": "ok", "provider": s.MDMProvider},
		},
	})
	logic.ReturnSuccessResponseWithJson(w, r, map[string]any{"provider": s.MDMProvider, "ok": true}, "mdm verify ok")
}

// @Summary     Trigger an out-of-cycle MDM sync
// @Router      /api/v1/mdm/sync [post]
// @Tags        MDM
// @Security    oauth
// @Produce     json
// @Success     202 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
func triggerMDMSync(w http.ResponseWriter, r *http.Request) {
	s := logic.GetServerSettings()
	if s.MDMProvider == models.MDMProviderDisabled {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("no MDM provider configured"), "badrequest"))
		return
	}
	go func() {
		if err := proLogic.RunMDMSyncForce(context.Background()); err != nil {
			logger.Log(0, "mdm: manual sync failed:", err.Error())
		}
	}()
	logic.LogEvent(&models.Event{
		Action:      schema.MDMSync,
		TriggeredBy: r.Header.Get("user"),
		Source: models.Subject{
			ID:   r.Header.Get("user"),
			Name: r.Header.Get("user"),
			Type: schema.UserSub,
		},
		Target: models.Subject{
			ID:   string(s.MDMProvider),
			Name: string(s.MDMProvider),
			Type: schema.MDMSub,
		},
		Origin: schema.Dashboard,
		Diff: models.Diff{
			New: map[string]interface{}{"status": "queued", "provider": s.MDMProvider},
		},
	})
	logic.ReturnSuccessResponseWithJson(w, r, map[string]any{"queued": true}, "mdm sync queued")
}

// @Summary     List synced MDM device states
// @Router      /api/v1/mdm/device_state [get]
// @Tags        MDM
// @Security    oauth
// @Produce     json
// @Param       host_id  query string false "Filter by host UUID"
// @Param       provider query string false "Filter by provider name"
// @Success     200 {array} schema.DeviceMDMState
func listMDMDeviceState(w http.ResponseWriter, r *http.Request) {
	ctx := db.WithContext(context.TODO())
	hostID := r.URL.Query().Get("host_id")
	provider := r.URL.Query().Get("provider")
	state := &schema.DeviceMDMState{HostID: hostID, Provider: provider}
	var out []schema.DeviceMDMState
	var err error
	switch {
	case hostID != "" && provider != "":
		if err = state.Get(ctx); err == nil {
			out = []schema.DeviceMDMState{*state}
		}
	case hostID != "":
		out, err = state.ListByHost(ctx)
	case provider != "":
		out, err = state.ListByProvider(ctx)
	default:
		out, err = state.ListAll(ctx)
	}
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "internal"))
		return
	}
	logic.ReturnSuccessResponseWithJson(w, r, out, "fetched mdm device states")
}

// Suppress unused import warnings when the build tag is flipped.
var _ = mux.Vars
