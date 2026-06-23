package scope

import (
	"context"
	"errors"
	"net/http"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/schema"
)

const (
	HeaderTenantID = "X-Tenant-ID"
	HeaderOrgID    = "X-Organization-ID"
)

// Scope represents the tenancy level of a request.
type Scope int

const (
	GlobalScope Scope = iota
	OrgScope
	TenantScope
)

type ctxKey int

const (
	ctxLevel ctxKey = iota
	ctxID
)

var (
	errMissingTenantID     = errors.New("X-Tenant-ID header is required")
	errTenantNotFound      = errors.New("tenant not found")
	errDefaultTenantFailed = errors.New("default tenant not found")
	errMissingOrgID        = errors.New("X-Organization-ID header is required")
	errOrgNotFound         = errors.New("organization not found")
	errDefaultOrgFailed    = errors.New("default organization not found")
)

// Middleware reads the scope header for the given level, validates the
// tenant/org, and stores the level and id in the request context.
//
// When AllowMultipleTenants is false and the header is absent, the default
// tenant/org is resolved and used. When AllowMultipleTenants is true, a
// missing header is a 400.
func Middleware(level Scope, next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var id string

		switch level {
		case TenantScope:
			id = r.Header.Get(HeaderTenantID)
			if id == "" {
				if logic.GetFeatureFlags().AllowMultipleTenants {
					logic.ReturnErrorResponse(w, r, logic.FormatError(errMissingTenantID, logic.BadReq))
					return
				}
				t := &schema.Tenant{}
				if err := t.GetDefault(r.Context()); err != nil {
					logic.ReturnErrorResponse(w, r, logic.FormatError(errDefaultTenantFailed, logic.Internal))
					return
				}
				id = t.ID
			} else {
				t := &schema.Tenant{ID: id}
				if err := t.Get(db.WithContext(r.Context())); err != nil {
					logic.ReturnErrorResponse(w, r, logic.FormatError(errTenantNotFound, logic.BadReq))
					return
				}
			}

		case OrgScope:
			id = r.Header.Get(HeaderOrgID)
			if id == "" {
				if logic.GetFeatureFlags().AllowMultipleTenants {
					logic.ReturnErrorResponse(w, r, logic.FormatError(errMissingOrgID, logic.BadReq))
					return
				}
				o := &schema.Organization{}
				if err := o.GetDefault(r.Context()); err != nil {
					logic.ReturnErrorResponse(w, r, logic.FormatError(errDefaultOrgFailed, logic.Internal))
					return
				}
				id = o.ID
			} else {
				o := &schema.Organization{ID: id}
				if err := o.Get(db.WithContext(r.Context())); err != nil {
					logic.ReturnErrorResponse(w, r, logic.FormatError(errOrgNotFound, logic.BadReq))
					return
				}
			}

		case GlobalScope:
			// no header required
		}

		next.ServeHTTP(w, r.WithContext(WithContext(r.Context(), level, id)))
	}
}

// WithContext stores the scope level and id in ctx and returns the new context.
// If level is non-global and id is empty, ctx is returned unchanged.
func WithContext(ctx context.Context, level Scope, id string) context.Context {
	if level != GlobalScope && id == "" {
		return ctx
	}
	ctx = context.WithValue(ctx, ctxLevel, level)
	ctx = context.WithValue(ctx, ctxID, id)
	return ctx
}

// Level returns the scope level stored in ctx, defaulting to GlobalScope.
func Level(ctx context.Context) Scope {
	v, _ := ctx.Value(ctxLevel).(Scope)
	return v
}

// ID returns the scope id stored in ctx.
func ID(ctx context.Context) string {
	v, _ := ctx.Value(ctxID).(string)
	return v
}
