package controller

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/middleware"
	"github.com/gravitl/netmaker/orchestrator"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func orgHandlers(r *mux.Router) {
	// TODO: add permissions check middleware
	r.HandleFunc("/api/v1/org/settings", middleware.Scope(scope.OrgScope, http.HandlerFunc(getOrgSettings))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/org/settings", middleware.Scope(scope.OrgScope, http.HandlerFunc(upsertOrgSettings))).Methods(http.MethodPut)

	r.HandleFunc("/api/v1/orgs", middleware.Scope(scope.GlobalScope, http.HandlerFunc(listOrgs))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/orgs", middleware.Scope(scope.GlobalScope, http.HandlerFunc(createOrg))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/orgs/{org_id}", middleware.Scope(scope.GlobalScope, http.HandlerFunc(getOrg))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/orgs/{org_id}", middleware.Scope(scope.GlobalScope, http.HandlerFunc(deleteOrg))).Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/orgs/{org_id}/owner", middleware.Scope(scope.GlobalScope, http.HandlerFunc(getOrgOwner))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/orgs/{org_id}/owner", middleware.Scope(scope.GlobalScope, http.HandlerFunc(createOrgOwner))).Methods(http.MethodPut)

	r.HandleFunc("/api/v1/tenants", middleware.Scope(scope.OrgScope, http.HandlerFunc(listTenants))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/tenants", middleware.Scope(scope.OrgScope, http.HandlerFunc(createTenant))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/tenants/{tenant_id}", middleware.Scope(scope.OrgScope, http.HandlerFunc(getTenant))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/tenants/{tenant_id}", middleware.Scope(scope.OrgScope, http.HandlerFunc(deleteTenant))).Methods(http.MethodDelete)
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

	logic.ResetAuthProvider(r.Context())

	if req.ClientSecret != "" {
		req.ClientSecret = logic.Mask()
	}
	logic.ReturnSuccessResponseWithJson(w, r, req, "updated org settings")
}

// @Summary     List organizations
// @Router      /api/v1/orgs [get]
// @Tags        Organizations
// @Security    oauth
// @Produce     json
// @Success     200 {array} schema.Organization
// @Failure     500 {object} models.ErrorResponse
func listOrgs(w http.ResponseWriter, r *http.Request) {
	o := &schema.Organization{}
	orgs, err := o.ListAll(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, orgs, "fetched organizations")
}

// @Summary     Create an organization
// @Router      /api/v1/orgs [post]
// @Tags        Organizations
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       body body schema.Organization true "Organization"
// @Success     200 {object} schema.Organization
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func createOrg(w http.ResponseWriter, r *http.Request) {
	var req schema.Organization
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	if req.Name == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("name is required"), logic.BadReq))
		return
	}

	o := &schema.Organization{
		Name:     req.Name,
		Metadata: req.Metadata,
	}
	err = o.Create(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, o, "created organization")
}

// @Summary     Get an organization
// @Router      /api/v1/orgs/{org_id} [get]
// @Tags        Organizations
// @Security    oauth
// @Produce     json
// @Param       org_id path string true "Organization ID"
// @Success     200 {object} schema.Organization
// @Failure     404 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func getOrg(w http.ResponseWriter, r *http.Request) {
	orgID := mux.Vars(r)["org_id"]

	o := &schema.Organization{ID: orgID}
	err := o.Get(r.Context())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("organization not found"), logic.NotFound))
			return
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, o, "fetched organization")
}

// @Summary     Get organization owner
// @Router      /api/v1/orgs/{org_id}/owner [get]
// @Tags        Organizations
// @Security    oauth
// @Produce     json
// @Param       org_id path string true "Organization ID"
// @Success     200 {object} models.ReturnUser
// @Failure     404 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func getOrgOwner(w http.ResponseWriter, r *http.Request) {
	orgID := mux.Vars(r)["org_id"]

	o := &schema.Organization{ID: orgID}
	err := o.Get(r.Context())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("organization not found"), logic.NotFound))
			return
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	membership := &schema.OrgMembership{OrganizationID: orgID}
	err = membership.GetOwner(r.Context())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("organization owner not found"), logic.NotFound))
			return
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	ctx := scope.WithContext(r.Context(), scope.OrgScope, orgID)
	owner := &schema.User{ID: membership.UserID}
	err = owner.Get(ctx)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, logic.ToReturnUser(owner), "fetched organization owner")
}

// @Summary     Create an organization owner
// @Router      /api/v1/orgs/{org_id}/owner [put]
// @Tags        Organizations
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       org_id path string true "Organization ID"
// @Param       body body schema.User true "Owner user details"
// @Success     200 {object} models.ReturnUser
// @Failure     400 {object} models.ErrorResponse
// @Failure     404 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func createOrgOwner(w http.ResponseWriter, r *http.Request) {
	orgID := mux.Vars(r)["org_id"]

	o := &schema.Organization{ID: orgID}
	err := o.Get(r.Context())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("organization not found"), logic.NotFound))
			return
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	var user schema.User
	err = json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	user.PlatformRoleID = schema.OrgOwner

	ctx := scope.WithContext(r.Context(), scope.OrgScope, orgID)

	err = orchestrator.GetRepository().UserOrchestrator().ValidateCreateUser(ctx, &user)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	err = orchestrator.GetRepository().UserOrchestrator().CreateUser(ctx, &user)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, logic.ToReturnUser(&user), "created organization owner")
}

// @Summary     Delete an organization
// @Router      /api/v1/orgs/{org_id} [delete]
// @Tags        Organizations
// @Security    oauth
// @Produce     json
// @Param       org_id path string true "Organization ID"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     404 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func deleteOrg(w http.ResponseWriter, r *http.Request) {
	orgID := mux.Vars(r)["org_id"]

	o := &schema.Organization{ID: orgID}
	err := o.Get(r.Context())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("organization not found"), logic.NotFound))
			return
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	t := &schema.Tenant{
		OrganizationID: orgID,
	}
	tenants, err := t.ListByOrg(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}
	if len(tenants) > 0 {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("cannot delete organization with existing tenants"), logic.BadReq))
		return
	}

	err = o.Delete(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	logic.ReturnSuccessResponse(w, r, "deleted organization")
}

// @Summary     List tenants in an organization
// @Router      /api/v1/tenants [get]
// @Tags        Tenants
// @Security    oauth
// @Produce     json
// @Success     200 {array} schema.Tenant
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func listTenants(w http.ResponseWriter, r *http.Request) {
	orgID := scope.ID(r.Context())

	t := &schema.Tenant{
		OrganizationID: orgID,
	}
	tenants, err := t.ListByOrg(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, tenants, "fetched tenants")
}

// @Summary     Create a tenant in an organization
// @Router      /api/v1/tenants [post]
// @Tags        Tenants
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       body body schema.Tenant true "Tenant"
// @Success     200 {object} schema.Tenant
// @Failure     400 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func createTenant(w http.ResponseWriter, r *http.Request) {
	orgID := scope.ID(r.Context())

	var req schema.Tenant
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	if req.Name == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("name is required"), logic.BadReq))
		return
	}

	t := &schema.Tenant{
		Name:           req.Name,
		OrganizationID: orgID,
		Metadata:       req.Metadata,
	}
	err = t.Create(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, t, "created tenant")
}

// @Summary     Get a tenant
// @Router      /api/v1/tenants/{tenant_id} [get]
// @Tags        Tenants
// @Security    oauth
// @Produce     json
// @Param       tenant_id path string true "Tenant ID"
// @Success     200 {object} schema.Tenant
// @Failure     400 {object} models.ErrorResponse
// @Failure     404 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func getTenant(w http.ResponseWriter, r *http.Request) {
	orgID := scope.ID(r.Context())
	if orgID == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("organization ID not in context"), logic.BadReq))
		return
	}
	tenantID := mux.Vars(r)["tenant_id"]

	t := &schema.Tenant{ID: tenantID}
	err := t.Get(r.Context())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("tenant not found"), logic.NotFound))
			return
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}
	if t.OrganizationID != orgID {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("tenant not found"), logic.NotFound))
		return
	}

	logic.ReturnSuccessResponseWithJson(w, r, t, "fetched tenant")
}

// @Summary     Delete a tenant
// @Router      /api/v1/tenants/{tenant_id} [delete]
// @Tags        Tenants
// @Security    oauth
// @Produce     json
// @Param       tenant_id path string true "Tenant ID"
// @Success     200 {object} models.SuccessResponse
// @Failure     400 {object} models.ErrorResponse
// @Failure     404 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func deleteTenant(w http.ResponseWriter, r *http.Request) {
	orgID := scope.ID(r.Context())
	tenantID := mux.Vars(r)["tenant_id"]

	t := &schema.Tenant{ID: tenantID}
	err := t.Get(r.Context())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("tenant not found"), logic.NotFound))
			return
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	if t.OrganizationID != orgID {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("tenant not found"), logic.NotFound))
		return
	}

	err = t.Delete(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	logic.ReturnSuccessResponse(w, r, "deleted tenant")
}
