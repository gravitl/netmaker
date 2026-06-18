package controller

import (
	"errors"
	"net/http"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/schema"
)

var (
	errMissingTenantID       = errors.New("X-Tenant-ID header is required")
	errDefaultTenantNotFound = errors.New("default tenant not found")
	errTenantNotFound        = errors.New("tenant not found")
	errMissingOrgID          = errors.New("X-Organization-ID header is required")
	errDefaultOrgNotFound    = errors.New("default organization not found")
	errOrgNotFound           = errors.New("organization not found")
)

const (
	HeaderTenantID = "X-Tenant-ID"
	HeaderOrgID    = "X-Organization-ID"
)

// Scope wraps an http.Handler to enforce request-level tenancy scoping.
//
// For db.TenantScope: requires the X-Tenant-ID header and injects a
// WHERE tenant_id = ? scope into the GORM db stored in the request context.
//
// For db.OrgScope: requires the X-Organization-ID header and injects a
// WHERE organization_id = ? scope.
//
// For db.GlobalScope: passes through without modification.
func Scope(level db.ScopeLevel, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var id string
		switch level {
		case db.TenantScope:
			id = r.Header.Get(HeaderTenantID)
			if id == "" {
				if logic.GetFeatureFlags().AllowMultipleTenants {
					logic.ReturnErrorResponse(w, r, logic.FormatError(errMissingTenantID, logic.BadReq))
					return
				}

				defaultTenant := &schema.Tenant{}
				err := defaultTenant.GetDefault(r.Context())
				if err != nil {
					logic.ReturnErrorResponse(w, r, logic.FormatError(errDefaultTenantNotFound, logic.Internal))
					return
				}

				id = defaultTenant.ID
			} else {
				tenant := &schema.Tenant{
					ID: id,
				}
				err := tenant.Get(r.Context())
				if err != nil {
					logic.ReturnErrorResponse(w, r, logic.FormatError(errTenantNotFound, logic.BadReq))
					return
				}
			}
		case db.OrgScope:
			id = r.Header.Get(HeaderOrgID)
			if id == "" {
				if logic.GetFeatureFlags().AllowMultipleTenants {
					logic.ReturnErrorResponse(w, r, logic.FormatError(errMissingOrgID, logic.BadReq))
					return
				}

				defaultOrg := &schema.Organization{}
				err := defaultOrg.Get(r.Context())
				if err != nil {
					logic.ReturnErrorResponse(w, r, logic.FormatError(errDefaultOrgNotFound, logic.Internal))
					return
				}

				id = defaultOrg.ID
			} else {
				org := &schema.Organization{
					ID: id,
				}
				err := org.Get(r.Context())
				if err != nil {
					logic.ReturnErrorResponse(w, r, logic.FormatError(errOrgNotFound, logic.BadReq))
					return
				}
			}
		case db.GlobalScope:
			// no header required
		}

		ctx := db.Scope(r.Context(), level, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
