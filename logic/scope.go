package logic

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
)

var defaultTenantID atomic.Value

// DefaultScope returns a context scoped to the default tenant.
// TODO: remove usage
func DefaultScope(ctx context.Context) context.Context {
	if defaultTenantID.Load() == nil {
		t := &schema.Tenant{}
		if err := t.GetDefault(ctx); err != nil {
			panic(fmt.Sprintf("scope: failed to resolve default tenant: %v", err))
		}

		defaultTenantID.Store(t.ID)
	}

	return scope.WithContext(ctx, scope.TenantScope, defaultTenantID.Load().(string))
}
