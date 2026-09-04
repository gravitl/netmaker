package utils

import (
	"context"
	"testing"

	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/require"
)

// CreateDefaultOrgAndTenant creates the default org/tenant, or loads them if a
// prior test run against the same (persistent) database already created them.
func CreateDefaultOrgAndTenant(t *testing.T, ctx context.Context) {
	defaultOrg := schema.Organization{}
	if err := defaultOrg.CreateDefault(ctx); err != nil {
		// CreateDefault sets ID before the failed insert; GetDefault must be
		// called on a fresh struct or gorm.First will (uselessly) AND the
		// stale ID into the WHERE clause and never match.
		defaultOrg = schema.Organization{}
		require.NoError(t, defaultOrg.GetDefault(ctx))
	}

	defaultTenant := schema.Tenant{
		OrganizationID: defaultOrg.ID,
	}
	if err := defaultTenant.CreateDefault(ctx); err != nil {
		defaultTenant = schema.Tenant{}
		require.NoError(t, defaultTenant.GetDefault(ctx))
	}
}
