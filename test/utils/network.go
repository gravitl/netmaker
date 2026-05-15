package utils

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/require"
)

func CreateIPv4Network(t *testing.T, name string) *schema.Network {
	addressRange, err := RandomPrivateCIDRv4(24)
	require.NoError(t, err)

	network := &schema.Network{
		ID:           uuid.NewString(),
		Name:         name,
		AddressRange: addressRange.String(),
	}
	err = network.Create(db.WithContext(context.TODO()))
	require.NoError(t, err)

	return network
}

func CreateIPv6Network(t *testing.T, name string) *schema.Network {
	addressRange6, err := RandomPrivateCIDRv6(48)
	require.NoError(t, err)

	network := &schema.Network{
		ID:            uuid.NewString(),
		Name:          name,
		AddressRange6: addressRange6.String(),
	}
	err = network.Create(db.WithContext(context.TODO()))
	require.NoError(t, err)

	return network
}

func CreateIPv10Network(t *testing.T, name string) *schema.Network {
	addressRange, err := RandomPrivateCIDRv4(24)
	require.NoError(t, err)
	addressRange6, err := RandomPrivateCIDRv6(48)
	require.NoError(t, err)

	network := &schema.Network{
		ID:            uuid.NewString(),
		Name:          name,
		AddressRange:  addressRange.String(),
		AddressRange6: addressRange6.String(),
	}
	err = network.Create(db.WithContext(context.TODO()))
	require.NoError(t, err)

	return network
}

func DeleteNetwork(t *testing.T, network *schema.Network) {
	err := network.Delete(db.WithContext(context.TODO()))
	require.NoError(t, err)
}
