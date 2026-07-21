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
// todo(nm-341): remove usage
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

func SoleOrganization(ctx context.Context) (*schema.Organization, error) {
	orgs, err := (&schema.Organization{}).ListAll(ctx)
	if err != nil {
		return nil, err
	}
	if len(orgs) != 1 {
		return nil, fmt.Errorf("expected exactly one organization, found %d", len(orgs))
	}
	return &orgs[0], nil
}

func SoleTenant(ctx context.Context) (*schema.Tenant, error) {
	tenants, err := (&schema.Tenant{}).List(ctx)
	if err != nil {
		return nil, err
	}
	if len(tenants) != 1 {
		return nil, fmt.Errorf("expected exactly one tenant, found %d", len(tenants))
	}
	return &tenants[0], nil
}
