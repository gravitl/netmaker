package middleware

import (
	"errors"
	"net/http"
	"sync/atomic"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
)

var (
	errMissingTenantID       = errors.New("X-Tenant-ID header is required")
	errTenantNotFound        = errors.New("tenant not found")
	errDefaultTenantNotFound = errors.New("default tenant not found")
	errMissingOrgID          = errors.New("X-Organization-ID header is required")
	errOrgNotFound           = errors.New("organization not found")
	errAmbiguousScope        = errors.New("only one of org or tenant scope header may be provided")
)

var defaultTenantID atomic.Value

// Scope reads the scope header for the given level, validates the tenant/org,
// and stores the level and id in the request context.
func Scope(level scope.Scope, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var id string

		switch level {
		case scope.TenantScope:
			id = r.Header.Get(scope.HeaderTenantID)
			if id == "" {
				if defaultTenantID.Load() == nil {
					t, err := logic.SoleTenant(r.Context())
					if err != nil {
						logic.ReturnErrorResponse(w, r, logic.FormatError(errDefaultTenantNotFound, logic.Internal))
						return
					}
					defaultTenantID.Store(t.ID)
				}

				id = defaultTenantID.Load().(string)
			} else {
				t := &schema.Tenant{ID: id}
				if err := t.Get(db.WithContext(r.Context())); err != nil {
					logic.ReturnErrorResponse(w, r, logic.FormatError(errTenantNotFound, logic.BadReq))
					return
				}
			}

		case scope.OrgScope:
			id = r.Header.Get(scope.HeaderOrgID)
			if id == "" {
				logic.ReturnErrorResponse(w, r, logic.FormatError(errMissingOrgID, logic.BadReq))
				return
			}

			o := &schema.Organization{ID: id}
			if err := o.Get(db.WithContext(r.Context())); err != nil {
				logic.ReturnErrorResponse(w, r, logic.FormatError(errOrgNotFound, logic.BadReq))
				return
			}

		case scope.GlobalScope:
			// no header required
		}

		next.ServeHTTP(w, r.WithContext(scope.WithContext(r.Context(), level, id)))
	}
}

// InferScope determines whether the request is tenant- or org-scoped based on
// which header is present, and scopes the request accordingly. It is intended for
// endpoints that support both tenant and org scope.
func InferScope(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID := r.Header.Get(scope.HeaderOrgID)
		tenantID := r.Header.Get(scope.HeaderTenantID)

		if orgID != "" && tenantID != "" {
			logic.ReturnErrorResponse(w, r, logic.FormatError(errAmbiguousScope, logic.BadReq))
			return
		}

		if orgID != "" {
			o := &schema.Organization{ID: orgID}
			if err := o.Get(db.WithContext(r.Context())); err != nil {
				logic.ReturnErrorResponse(w, r, logic.FormatError(errOrgNotFound, logic.BadReq))
				return
			}
			next.ServeHTTP(w, r.WithContext(scope.WithContext(r.Context(), scope.OrgScope, orgID)))
			return
		}

		if tenantID != "" {
			t := &schema.Tenant{ID: tenantID}
			if err := t.Get(db.WithContext(r.Context())); err != nil {
				logic.ReturnErrorResponse(w, r, logic.FormatError(errTenantNotFound, logic.BadReq))
				return
			}
		} else {
			if defaultTenantID.Load() == nil {
				t, err := logic.SoleTenant(r.Context())
				if err != nil {
					logic.ReturnErrorResponse(w, r, logic.FormatError(errDefaultTenantNotFound, logic.Internal))
					return
				}
				defaultTenantID.Store(t.ID)
			}
			tenantID = defaultTenantID.Load().(string)
		}

		next.ServeHTTP(w, r.WithContext(scope.WithContext(r.Context(), scope.TenantScope, tenantID)))
	}
}
