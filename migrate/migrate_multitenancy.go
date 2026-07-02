package migrate

import (
	"context"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
)

func migrateMultiTenancy(ctx context.Context) error {
	return createDefaults(ctx)
}

func createDefaults(ctx context.Context) error {
	// skip default org and tenant creation if a new deployment.
	if db.FromContext(ctx).Migrator().HasTable(TableName_Users) {
		numUsers, err := kvCount(ctx, TableName_Users)
		if err != nil {
			return err
		}

		if numUsers == 0 {
			numUsers, err = (&schema.User{}).Count(ctx)
			if err != nil {
				return err
			}

			if numUsers == 0 {
				return nil
			}
		}
	} else {
		numUsers, err := (&schema.User{}).Count(ctx)
		if err != nil {
			return err
		}

		if numUsers == 0 {
			return nil
		}
	}

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
