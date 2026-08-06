package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/db"
	dbtypes "github.com/gravitl/netmaker/db/types"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/middleware"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/orchestrator"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func orgHandlers(r *mux.Router) {
	r.HandleFunc("/api/v1/org/settings", middleware.Scope(scope.OrgScope, logic.SecurityCheck(true, http.HandlerFunc(getOrgSettings)))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/org/settings", middleware.Scope(scope.OrgScope, logic.SecurityCheck(true, http.HandlerFunc(upsertOrgSettings)))).Methods(http.MethodPut)
	r.HandleFunc("/api/v1/org/owner/transfer", middleware.Scope(scope.OrgScope, logic.SecurityCheck(true, http.HandlerFunc(transferOrgOwner)))).Methods(http.MethodPut)

	r.HandleFunc("/api/v1/orgs", middleware.Scope(scope.GlobalScope, logic.SecurityCheck(true, http.HandlerFunc(listOrgs)))).Methods(http.MethodGet)
	//r.HandleFunc("/api/v1/orgs", middleware.Scope(scope.GlobalScope, http.HandlerFunc(createOrg))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/orgs/{org_id}", middleware.Scope(scope.GlobalScope, logic.SecurityCheck(true, http.HandlerFunc(getOrg)))).Methods(http.MethodGet)
	//r.HandleFunc("/api/v1/orgs/{org_id}", middleware.Scope(scope.GlobalScope, http.HandlerFunc(deleteOrg))).Methods(http.MethodDelete)
	r.HandleFunc("/api/v1/orgs/{org_id}/owner", middleware.Scope(scope.GlobalScope, http.HandlerFunc(getOrgOwner))).Methods(http.MethodGet)
	r.HandleFunc("/api/v1/orgs/{org_id}/owner", middleware.Scope(scope.GlobalScope, http.HandlerFunc(createOrgOwner))).Methods(http.MethodPut)

	r.HandleFunc("/api/v1/tenants", middleware.Scope(scope.OrgScope, logic.SecurityCheck(true, http.HandlerFunc(listTenants)))).Methods(http.MethodGet)
	//r.HandleFunc("/api/v1/tenants", middleware.Scope(scope.OrgScope, logic.SecurityCheck(true, http.HandlerFunc(createTenant)))).Methods(http.MethodPost)
	r.HandleFunc("/api/v1/tenants/{tenant_id}", middleware.Scope(scope.GlobalScope, http.HandlerFunc(getTenant))).Methods(http.MethodGet)
	//r.HandleFunc("/api/v1/tenants/{tenant_id}", middleware.Scope(scope.OrgScope, logic.SecurityCheck(true, http.HandlerFunc(deleteTenant)))).Methods(http.MethodDelete)
}

var errOrgNotFound = errors.New("organization not found")

func resolveSoleOrg(ctx context.Context, orgID string) (*schema.Organization, error) {
	o, err := logic.SoleOrganization(ctx)
	if err != nil {
		return nil, err
	}

	if orgID == "sole" {
		return o, nil
	}

	if orgID == o.ID {
		return o, nil
	}

	if orgID == o.Slug {
		return o, nil
	}

	return nil, errOrgNotFound
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
	if data.EmailSenderPassword != "" {
		data.EmailSenderPassword = logic.Mask()
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
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			err = fmt.Errorf("failed to upsert org %s settings: error checking for existing settings: %v", orgID, err)
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
			return
		}
	} else {
		if req.AuthProvider != existing.Settings.Data().AuthProvider && req.AuthProvider == "" {
			om := &schema.OrgMembership{
				OrganizationID: orgID,
			}
			err = om.GetOwner(r.Context())
			if err != nil {
				err = fmt.Errorf("failed to upsert org %s settings: error getting org owner: %v", orgID, err)
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
				return
			}

			if om.AuthType == schema.OAuth {
				err := fmt.Errorf("cannot remove Oauth integration because an OAuth user has the org-owner role")
				logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
				return
			}
		}
	}

	if req.ClientSecret == logic.Mask() {
		req.ClientSecret = existing.Settings.Data().ClientSecret
	}
	if req.EmailSenderPassword == logic.Mask() {
		req.EmailSenderPassword = existing.Settings.Data().EmailSenderPassword
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
	logic.EmailInit(scope.WithContext(db.WithContext(context.Background()), scope.OrgScope, orgID))

	if req.ClientSecret != "" {
		req.ClientSecret = logic.Mask()
	}
	if req.EmailSenderPassword != "" {
		req.EmailSenderPassword = logic.Mask()
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

	if r.Header.Get("ismaster") == "yes" {
		orgs, err := o.ListAll(r.Context())
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
			return
		}
		logic.ReturnSuccessResponseWithJson(w, r, orgs, "fetched organizations")
		return
	}

	u := &schema.User{Username: r.Header.Get("user")}
	err := u.Get(r.Context())
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	orgs, err := o.ListOrgsByUserID(r.Context(), u.ID)
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
//func createOrg(w http.ResponseWriter, r *http.Request) {
//	var req schema.Organization
//	err := json.NewDecoder(r.Body).Decode(&req)
//	if err != nil {
//		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
//		return
//	}
//
//	if req.Name == "" {
//		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("name is required"), logic.BadReq))
//		return
//	}
//
//	o := &schema.Organization{
//		Name:     req.Name,
//		Metadata: req.Metadata,
//	}
//	err = o.Create(r.Context())
//	if err != nil {
//		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
//		return
//	}
//
//	logic.ReturnSuccessResponseWithJson(w, r, o, "created organization")
//}

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

	o := &schema.Organization{ID: orgID, Slug: orgID}
	err := o.Get(r.Context())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("organization not found"), logic.NotFound))
			return
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	if r.Header.Get("ismaster") != "yes" {
		u := &schema.User{Username: r.Header.Get("user")}
		err = u.Get(r.Context())
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
			return
		}

		m := &schema.OrgMembership{OrganizationID: o.ID, UserID: u.ID}
		err = m.Get(r.Context())
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("not a member of this organization"), logic.Forbidden))
				return
			}
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
			return
		}
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

	o, err := resolveSoleOrg(r.Context(), orgID)
	if err != nil {
		if errors.Is(err, errOrgNotFound) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("organization not found"), logic.NotFound))
			return
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	membership := &schema.OrgMembership{OrganizationID: o.ID}
	err = membership.GetOwner(r.Context())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("organization owner not found"), logic.NotFound))
			return
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	ctx := scope.WithContext(r.Context(), scope.OrgScope, o.ID)
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

	dbctx := db.BeginTx(r.Context())
	commit := false
	defer func() {
		if commit {
			db.FromContext(dbctx).Commit()
		} else {
			db.FromContext(dbctx).Rollback()
		}
	}()

	o, err := resolveSoleOrg(dbctx, orgID)
	if err != nil {
		if errors.Is(err, errOrgNotFound) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("organization not found"), logic.NotFound))
			return
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	existingOwner := &schema.OrgMembership{OrganizationID: o.ID}
	err = existingOwner.GetOwner(dbctx)
	if err == nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("organization owner already exists"), logic.BadReq))
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	var user schema.User
	err = json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	authParams := models.UserAuthParams{
		UserName: user.Username,
		Password: user.Password,
	}

	user.PlatformRoleID = schema.OrgOwner

	ctx := scope.WithContext(dbctx, scope.OrgScope, o.ID)

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

	tenants, err := (&schema.Tenant{}).List(dbctx, dbtypes.WithFilter("organization_id", o.ID))
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	for _, tenant := range tenants {
		err = orchestrator.GetRepository().TenantOrchestrator().GrantTenantSuperAdmin(dbctx, tenant.ID, &user)
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
			return
		}
	}

	authToken, err := logic.VerifyAuthRequest(ctx, authParams, logic.DashboardApp)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	commit = true

	type userWithToken struct {
		models.ReturnUser
		AuthToken string `json:"auth_token"`
	}

	response := userWithToken{
		ReturnUser: logic.ToReturnUser(&user),
		AuthToken:  authToken,
	}

	logic.ReturnSuccessResponseWithJson(w, r, response, "created organization owner")
}

// @Summary     Transfer organization ownership
// @Router      /api/v1/org/owner/transfer [put]
// @Tags        Organizations
// @Security    oauth
// @Accept      json
// @Produce     json
// @Param       body body models.TransferOrgOwnerRequest true "Username of the org admin to transfer ownership to"
// @Success     200 {object} models.ReturnUser
// @Failure     400 {object} models.ErrorResponse
// @Failure     403 {object} models.ErrorResponse
// @Failure     404 {object} models.ErrorResponse
// @Failure     500 {object} models.ErrorResponse
func transferOrgOwner(w http.ResponseWriter, r *http.Request) {
	var req models.TransferOrgOwnerRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}
	if req.Username == "" {
		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("username is required"), logic.BadReq))
		return
	}

	dbctx := db.BeginTx(r.Context())
	commit := false
	defer func() {
		if commit {
			db.FromContext(dbctx).Commit()
		} else {
			db.FromContext(dbctx).Rollback()
		}
	}()

	caller := &schema.User{Username: r.Header.Get("user")}
	err = caller.GetWithMembership(dbctx)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}
	if caller.PlatformRoleID != schema.OrgOwner {
		err = errors.New("only the organization owner can transfer ownership")
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Forbidden))
		return
	}

	newOwner := &schema.User{Username: req.Username}
	err = newOwner.GetWithMembership(dbctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("user not found"), logic.NotFound))
			return
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}
	if newOwner.PlatformRoleID != schema.OrgAdmin {
		err = errors.New("only an organization admin can be promoted to organization owner")
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
		return
	}

	newOwner.PlatformRoleID = schema.OrgOwner
	err = (&schema.OrgMembership{
		OrganizationID: scope.ID(r.Context()),
		UserID:         newOwner.ID,
		RoleID:         newOwner.PlatformRoleID,
	}).UpdateRoleID(dbctx)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	caller.PlatformRoleID = schema.OrgAdmin
	err = (&schema.OrgMembership{
		OrganizationID: scope.ID(r.Context()),
		UserID:         caller.ID,
		RoleID:         caller.PlatformRoleID,
	}).UpdateRoleID(dbctx)
	if err != nil {
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
		return
	}

	commit = true

	logic.ReturnSuccessResponseWithJson(w, r, logic.ToReturnUser(newOwner), "transferred organization ownership")
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
//func deleteOrg(w http.ResponseWriter, r *http.Request) {
//	orgID := mux.Vars(r)["org_id"]
//
//	o := &schema.Organization{ID: orgID, Slug: orgID}
//	err := o.Get(r.Context())
//	if err != nil {
//		if errors.Is(err, gorm.ErrRecordNotFound) {
//			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("organization not found"), logic.NotFound))
//			return
//		}
//		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
//		return
//	}
//
//	tenants, err := (&schema.Tenant{}).List(r.Context(), dbtypes.WithFilter("organization_id", orgID))
//	if err != nil {
//		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
//		return
//	}
//	if len(tenants) > 0 {
//		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("cannot delete organization with existing tenants"), logic.BadReq))
//		return
//	}
//
//	err = o.Delete(r.Context())
//	if err != nil {
//		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
//		return
//	}
//
//	logic.ReturnSuccessResponse(w, r, "deleted organization")
//}

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

	tenants, err := (&schema.Tenant{}).List(r.Context(), dbtypes.WithFilter("organization_id", orgID))
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
//func createTenant(w http.ResponseWriter, r *http.Request) {
//	orgID := scope.ID(r.Context())
//	username := r.Header.Get("user")
//
//	user := &schema.User{Username: username}
//	err := user.Get(r.Context())
//	if err != nil {
//		err = fmt.Errorf("error getting user %s: %w", username, err)
//		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
//		return
//	}
//
//	var req schema.Tenant
//	err = json.NewDecoder(r.Body).Decode(&req)
//	if err != nil {
//		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
//		return
//	}
//
//	if req.Name == "" {
//		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("name is required"), logic.BadReq))
//		return
//	}
//
//	if req.Slug == schema.DefaultTenantSlug {
//		err = fmt.Errorf("cannot use slug '%s'", schema.DefaultTenantSlug)
//		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.BadReq))
//		return
//	}
//
//	t := &schema.Tenant{
//		Name:           req.Name,
//		Slug:           req.Slug,
//		OrganizationID: orgID,
//		Metadata:       req.Metadata,
//	}
//	err = t.Create(r.Context())
//	if err != nil {
//		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
//		return
//	}
//
//	user.PlatformRoleID = schema.SuperAdminRole
//	ctx := scope.WithContext(r.Context(), scope.TenantScope, t.ID)
//	err = orchestrator.GetRepository().UserOrchestrator().CreateUser(ctx, user, orchestrator.WithInheritedAuth())
//	if err != nil {
//		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
//		return
//	}
//
//	logic.ReturnSuccessResponseWithJson(w, r, t, "created tenant")
//}

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
	tenantID := mux.Vars(r)["tenant_id"]

	t := &schema.Tenant{ID: tenantID, Slug: tenantID}
	err := t.Get(r.Context())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("tenant not found"), logic.NotFound))
			return
		}
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
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
//func deleteTenant(w http.ResponseWriter, r *http.Request) {
//	orgID := scope.ID(r.Context())
//	tenantID := mux.Vars(r)["tenant_id"]
//
//	t := &schema.Tenant{ID: tenantID, Slug: tenantID}
//	err := t.Get(r.Context())
//	if err != nil {
//		if errors.Is(err, gorm.ErrRecordNotFound) {
//			logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("tenant not found"), logic.NotFound))
//			return
//		}
//		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
//		return
//	}
//
//	if t.OrganizationID != orgID {
//		logic.ReturnErrorResponse(w, r, logic.FormatError(errors.New("tenant not found"), logic.NotFound))
//		return
//	}
//
//	err = t.Delete(r.Context())
//	if err != nil {
//		logic.ReturnErrorResponse(w, r, logic.FormatError(err, logic.Internal))
//		return
//	}
//
//	logic.ReturnSuccessResponse(w, r, "deleted tenant")
//}
