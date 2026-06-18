package db

import (
	"context"

	"gorm.io/gorm"
)

// ScopeLevel represents the tenancy scope of a request.
type ScopeLevel int

const (
	// GlobalScope applies no tenant filtering — raw, unscoped access.
	GlobalScope ScopeLevel = iota
	// OrgScope filters queries to a specific organization (WHERE organization_id = ?).
	OrgScope
	// TenantScope filters queries to a specific tenant (WHERE tenant_id = ?).
	TenantScope
)

// Scope returns a new context whose GORM db is scoped to the given level.
//
// For OrgScope and TenantScope, exactly one id must be provided.
// For GlobalScope, no id is needed; the db is returned unscoped.
//
// Panics on invalid usage (wrong number of ids). These call sites are always
// static, so invalid usage is caught during development and code review.
func Scope(ctx context.Context, level ScopeLevel, ids ...string) context.Context {
	if len(ids) > 1 {
		panic("db.Scope: at most one id is allowed")
	}
	if level != GlobalScope && len(ids) == 0 {
		panic("db.Scope: id required for non-global scope")
	}
	if level == GlobalScope {
		return ctx
	}
	gdb := FromContext(ctx)
	switch level {
	case TenantScope:
		gdb = gdb.Scopes(func(db *gorm.DB) *gorm.DB {
			return db.Where("tenant_id = ?", ids[0])
		})
	case OrgScope:
		gdb = gdb.Scopes(func(db *gorm.DB) *gorm.DB {
			return db.Where("organization_id = ?", ids[0])
		})
	default:
		panic("db.Scope: unknown level")
	}
	return context.WithValue(ctx, dbCtxKey, gdb)
}
