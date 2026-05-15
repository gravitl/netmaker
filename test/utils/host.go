package utils

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/require"
)

func CreateHost(t *testing.T, name string) *schema.Host {
	endpointIP, err := RandomPublicIPv4()
	require.NoError(t, err)

	endpointIPv6, err := RandomPublicIPv6()
	require.NoError(t, err)

	host := &schema.Host{
		ID:           uuid.New(),
		Name:         name,
		EndpointIP:   endpointIP,
		EndpointIPv6: endpointIPv6,
	}
	err = host.Create(db.WithContext(context.TODO()))
	require.NoError(t, err)

	return host
}

func DeleteHost(t *testing.T, host *schema.Host) {
	err := host.Delete(db.WithContext(context.TODO()))
	require.NoError(t, err)
}
