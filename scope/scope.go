package scope

import (
	"context"
	"errors"
	"net/http"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/gorm"
)

const (
	HeaderTenantID = "X-Tenant-ID"
	HeaderOrgID    = "X-Organization-ID"
)

// Level represents the tenancy scope of a request.
type Level int

const (
	// GlobalScope applies no tenant filtering — raw, unscoped access.
	GlobalScope Level = iota
	// OrgScope filters queries to a specific organization (WHERE organization_id = ?).
	OrgScope
	// TenantScope filters queries to a specific tenant (WHERE tenant_id = ?).
	TenantScope
)

type scopeCtxKeyType int

const (
	scopeLevel scopeCtxKeyType = iota
	scopeID
)

var (
	errMissingTenantID       = errors.New("X-Tenant-ID header is required")
	errDefaultTenantNotFound = errors.New("default tenant not found")
	errTenantNotFound        = errors.New("tenant not found")
	errMissingOrgID          = errors.New("X-Organization-ID header is required")
	errDefaultOrgNotFound    = errors.New("default organization not found")
	errOrgNotFound           = errors.New("organization not found")
)

// Middleware wraps an http.Handler to enforce request-level tenancy scoping.
//
// For db.TenantScope: requires the X-Tenant-ID header and injects a
// WHERE tenant_id = ? scope into the GORM db stored in the request context.
//
// For db.OrgScope: requires the X-Organization-ID header and injects a
// WHERE organization_id = ? scope.
//
// For db.GlobalScope: passes through without modification.
func Middleware(level Level, next http.Handler) http.HandlerFunc {
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
		case OrgScope:
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
		case GlobalScope:
			// no header required
		}

		ctx := WithContext(r.Context(), level, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// WithContext returns a new context with GORM handle that is scoped to the given level.
//
// For OrgScope and TenantScope, exactly one id must be provided.
// For GlobalScope, no id is needed; the db is returned unscoped.
//
// Panics on invalid usage (wrong number of ids). These call sites are always
// static, so invalid usage is caught during development and code review.
func WithContext(ctx context.Context, level Level, ids ...string) context.Context {
	if len(ids) > 1 {
		panic("db.Scope: at most one id is allowed")
	}
	if level != GlobalScope && len(ids) == 0 {
		panic("db.Scope: id required for non-global scope")
	}

	ctx = context.WithValue(ctx, scopeLevel, level)
	ctx = context.WithValue(ctx, scopeID, ids[0])

	switch level {
	case TenantScope:
		return db.Modify(ctx, func(db *gorm.DB) *gorm.DB {
			return db.Scopes(func(db *gorm.DB) *gorm.DB {
				return db.Where("tenant_id = ?", ids[0])
			})
		})
	case OrgScope:
		return db.Modify(ctx, func(db *gorm.DB) *gorm.DB {
			return db.Scopes(func(db *gorm.DB) *gorm.DB {
				return db.Where("organization_id = ?", ids[0])
			})
		})
	case GlobalScope:
		return db.Modify(ctx, func(db *gorm.DB) *gorm.DB { return db })
	default:
		panic("db.Scope: unknown level")
	}
}

// Default returns a default tenant context.
// TODO: this is a temporary function. remove it and all it's usages.
// TODO: tenant context setting MUST be explicit.
func Default(ctx context.Context) context.Context {
	defaultTenant := &schema.Tenant{}
	err := defaultTenant.GetDefault(db.WithContext(context.TODO()))
	if err != nil {
		return ctx
	}

	return WithContext(ctx, TenantScope, defaultTenant.ID)
}

func ID(ctx context.Context) string {
	return ctx.Value(scopeID).(string)
}
