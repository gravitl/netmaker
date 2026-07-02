package controllers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/middleware"
	proAuth "github.com/gravitl/netmaker/pro/auth"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func OrgHandlers(r *mux.Router) {
	// TODO: add permissions check middleware
	r.HandleFunc("/api/v1/org/settings", middleware.Scope(scope.OrgScope, http.HandlerFunc(getOrgSettings))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/org/settings", middleware.Scope(scope.OrgScope, http.HandlerFunc(upsertOrgSettings))).Methods(http.MethodPut)
}

// @Summary     Get organization SSO settings
// @Router      /api/v1/org/settings [get]
// @Tags        Organizations
// @Security    oauth
// @Produce     json
// @Success     200 {object} schema.OrganizationSettingsData
// @Failure     500 {object} models.ErrorResponse
func getOrgSettings(w http.ResponseWriter, r *http.Request) {
	orgID := scope.ID(r.Context())
	if orgID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("organization ID not in context"), logic.BadReq))
		return
	}

	os := &schema.OrganizationSettings{ID: orgID}
	err := os.Get(r.Context())
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	data := os.Settings.Data()
	if data.ClientSecret != "" {
		data.ClientSecret = logic.Mask()
	}

	logic.ReturnSuccessResponseWithJson(w, r, data, "fetched org settings")
}

// @Summary     Update organization SSO settings
// @Router      /api/v1/org/settings [put]
// @Tags        Organizations
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       body body schema.OrganizationSettingsData true "Organization settings"
// @Success     200 {object} schema.OrganizationSettingsData
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func upsertOrgSettings(w http.ResponseWriter, r *http.Request) {
	orgID := scope.ID(r.Context())
	if orgID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("organization ID not in context"), logic.BadReq))
		return
	}

	var req schema.OrganizationSettingsData
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	existing := &schema.OrganizationSettings{ID: orgID}
	err = existing.Get(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	if req.ClientSecret == logic.Mask() {
		req.ClientSecret = existing.Settings.Data().ClientSecret
	}

	os := &schema.OrganizationSettings{
		ID:       orgID,
		Settings: datatypes.NewJSONType(req),
	}
	err = os.Upsert(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	proAuth.ResetAuthProvider(r.Context())

	if req.ClientSecret != "" {
		req.ClientSecret = logic.Mask()
	}
	logic.ReturnSuccessResponseWithJson(w, r, req, "updated org settings")
}
