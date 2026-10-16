package logic

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtClientRsrcName(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, db.InitializeDB(schema.ListModels()...))
	t.Cleanup(db.CloseDB)

	ctx := scope.WithContext(db.WithContext(context.Background()), scope.TenantScope, "tenant-1")
	network := &schema.Network{TenantID: "tenant-1", Name: "net-1", AddressRange: "10.0.0.0/24"}
	require.NoError(t, network.Create(ctx))
	extclient := &models.ExtClient{ClientID: "client-1", Network: network.Name}
	require.NoError(t, logic.SaveExtClient(ctx, extclient))

	extClientsRsrc := schema.ExtClientsRsrc.String()
	assert.Equal(t, "client-1", extClientRsrcName(ctx, extClientsRsrc, extclient.ID, network.Name), "ids resolve to names")
	assert.Equal(t, "client-1", extClientRsrcName(ctx, extClientsRsrc, "client-1", network.Name), "names are kept")

	unknownID := uuid.NewString()
	assert.Equal(t, unknownID, extClientRsrcName(ctx, extClientsRsrc, unknownID, network.Name), "unknown ids are kept")
	assert.Equal(t, extclient.ID, extClientRsrcName(ctx, schema.NetworkRsrc.String(), extclient.ID, network.Name), "other resources are kept")
}
