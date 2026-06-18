package controller

import (
	"errors"
	"net/http"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
)

var (
	errMissingTenantID = errors.New("X-Tenant-ID header is required")
	errMissingOrgID    = errors.New("X-Organization-ID header is required")
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
				logic.ReturnErrorResponse(w, r, logic.FormatError(errMissingTenantID, logic.BadReq))
				return
			}
		case db.OrgScope:
			id = r.Header.Get(HeaderOrgID)
			if id == "" {
				logic.ReturnErrorResponse(w, r, logic.FormatError(errMissingOrgID, logic.BadReq))
				return
			}
		case db.GlobalScope:
			// no header required
		}

		ctx := db.Scope(r.Context(), level, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
