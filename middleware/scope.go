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
	errMissingTenantID     = errors.New("X-Tenant-ID header is required")
	errTenantNotFound      = errors.New("tenant not found")
	errDefaultTenantFailed = errors.New("default tenant not found")
	errMissingOrgID        = errors.New("X-Organization-ID header is required")
	errOrgNotFound         = errors.New("organization not found")
	errDefaultOrgFailed    = errors.New("default organization not found")
)

var (
	defaultTenantID atomic.Value
	defaultOrgID    atomic.Value
)

// Scope reads the scope header for the given level, validates the tenant/org,
// and stores the level and id in the request context.
func Scope(level scope.Scope, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var id string

		switch level {
		case scope.TenantScope:
			id = r.Header.Get(scope.HeaderTenantID)
			if id == "" {
				if logic.GetFeatureFlags().AllowMultipleTenants {
					logic.ReturnErrorResponse(w, r, logic.FormatError(errMissingTenantID, logic.BadReq))
					return
				}

				if defaultTenantID.Load() == nil {
					t := &schema.Tenant{}
					if err := t.GetDefault(r.Context()); err != nil {
						logic.ReturnErrorResponse(w, r, logic.FormatError(errDefaultTenantFailed, logic.Internal))
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
				if logic.GetFeatureFlags().AllowMultipleTenants {
					logic.ReturnErrorResponse(w, r, logic.FormatError(errMissingOrgID, logic.BadReq))
					return
				}

				if defaultOrgID.Load() == nil {
					o := &schema.Organization{}
					if err := o.GetDefault(r.Context()); err != nil {
						logic.ReturnErrorResponse(w, r, logic.FormatError(errDefaultOrgFailed, logic.Internal))
						return
					}
					defaultOrgID.Store(o.ID)
				}

				id = defaultOrgID.Load().(string)
			} else {
				o := &schema.Organization{ID: id}
				if err := o.Get(db.WithContext(r.Context())); err != nil {
					logic.ReturnErrorResponse(w, r, logic.FormatError(errOrgNotFound, logic.BadReq))
					return
				}
			}

		case scope.GlobalScope:
			// no header required
		}

		next.ServeHTTP(w, r.WithContext(scope.WithContext(r.Context(), level, id)))
	}
}
