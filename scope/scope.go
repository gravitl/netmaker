package scope

import "context"

const (
	HeaderTenantID = "X-Tenant-ID"
	HeaderOrgID    = "X-Organization-ID"
)

var defaultTenantID atomic.Value

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

// Default returns a context scoped to the default tenant.
// TODO: remove usage
func Default(ctx context.Context) context.Context {
	if defaultTenantID.Load() == nil {
		t := &schema.Tenant{}
		if err := t.GetDefault(ctx); err != nil {
			panic(fmt.Sprintf("scope: failed to resolve default tenant: %v", err))
		}

		defaultTenantID.Store(t.ID)
	}

	return WithContext(ctx, TenantScope, defaultTenantID.Load().(string))
}
