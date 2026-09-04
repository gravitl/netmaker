package utils

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/stretchr/testify/require"
)

func CreateHost(t *testing.T, ctx context.Context, name string) *schema.Host {
	endpointIP, err := RandomPublicIPv4()
	require.NoError(t, err)

	endpointIPv6, err := RandomPublicIPv6()
	require.NoError(t, err)

	host := &schema.Host{
		ID:           uuid.New(),
		TenantID:     scope.ID(ctx),
		Name:         name,
		EndpointIP:   endpointIP,
		EndpointIPv6: endpointIPv6,
	}
	err = host.Create(ctx)
	require.NoError(t, err)

	return host
}

func DeleteHost(t *testing.T, ctx context.Context, host *schema.Host) {
	err := host.Delete(ctx)
	require.NoError(t, err)
}
