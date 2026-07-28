package logic

import (
	"context"
	"fmt"

	"github.com/gravitl/netmaker/schema"
)

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
