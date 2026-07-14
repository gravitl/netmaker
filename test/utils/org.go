package utils

import (
	"context"
	"testing"

	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/require"
)

func CreateDefaultOrgAndTenant(t *testing.T, ctx context.Context) {
	defaultOrg := schema.Organization{}
	err := defaultOrg.CreateDefault(ctx)
	require.NoError(t, err)

	defaultTenant := schema.Tenant{
		OrganizationID: defaultOrg.ID,
	}
	err = defaultTenant.CreateDefault(ctx)
	require.NoError(t, err)
}
