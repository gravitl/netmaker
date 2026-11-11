package logic

import (
	"context"
	"testing"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDisconnectUserExtClientsDisablesStaticConfigs(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, db.InitializeDB(schema.ListModels()...))
	t.Cleanup(db.CloseDB)

	ctx := scope.WithContext(db.WithContext(context.Background()), scope.TenantScope, "tenant-1")
	network := &schema.Network{TenantID: "tenant-1", Name: "jit-ext-net", AddressRange: "10.10.0.0/24"}
	require.NoError(t, network.Create(ctx))

	staticCfg := &models.ExtClient{
		ClientID: "static-cfg",
		Network:  network.Name,
		OwnerID:  "alice",
		Enabled:  true,
	}
	require.NoError(t, logic.SaveExtClient(ctx, staticCfg))

	racClient := &models.ExtClient{
		ClientID:             "rac-client",
		Network:              network.Name,
		OwnerID:              "alice",
		Enabled:              true,
		DeviceID:             "device-1",
		RemoteAccessClientID: "rac-1",
	}
	require.NoError(t, logic.SaveExtClient(ctx, racClient))

	otherOwner := &models.ExtClient{
		ClientID:             "other-rac",
		Network:              network.Name,
		OwnerID:              "bob",
		Enabled:              true,
		RemoteAccessClientID: "rac-2",
	}
	require.NoError(t, logic.SaveExtClient(ctx, otherOwner))

	require.NoError(t, disconnectUserExtClients(ctx, network.Name, "alice"))

	got, err := logic.GetExtClient(ctx, "static-cfg", network.Name)
	require.NoError(t, err, "static config file should be kept")
	assert.False(t, got.Enabled, "static config should be disabled")

	_, err = logic.GetExtClient(ctx, "rac-client", network.Name)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "RAC client should be deleted")

	_, err = logic.GetExtClient(ctx, "other-rac", network.Name)
	require.NoError(t, err, "other owners should be untouched")
}
