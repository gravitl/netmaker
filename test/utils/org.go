package utils

import (
	"context"
	"testing"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/require"
)

func CreateDefaultOrgAndTenant(t *testing.T) {
	defaultOrg := schema.Organization{}
	err := defaultOrg.CreateDefault(db.WithContext(context.TODO()))
	require.NoError(t, err)

	defaultTenant := schema.Tenant{
		OrganizationID: defaultOrg.ID,
	}
	err = defaultTenant.CreateDefault(db.WithContext(context.TODO()))
	require.NoError(t, err)
}
