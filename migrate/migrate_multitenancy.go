package migrate

import (
	"context"

	"github.com/gravitl/netmaker/schema"
)

func migrateMultiTenancy(ctx context.Context) error {
	return createDefaults(ctx)
}

func createDefaults(ctx context.Context) error {
	defaultOrg := &schema.Organization{}
	err := defaultOrg.CreateDefault(ctx)
	if err != nil {
		return err
	}

	defaultTenant := &schema.Tenant{
		OrganizationID: defaultOrg.ID,
	}
	return defaultTenant.CreateDefault(ctx)
}
