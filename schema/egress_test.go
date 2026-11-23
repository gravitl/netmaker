package schema_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEgressCreatePersistsBypassFalse(t *testing.T) {
	ctx := setupSchemaTest(t)

	disabled := &schema.Egress{
		ID:                 uuid.NewString(),
		TenantID:           "tenant-1",
		Name:               "exit-legacy",
		Network:            "net-1",
		Type:               schema.EgressTypeInternet,
		Range:              "*",
		BypassEgressRoutes: false,
		Status:             true,
	}
	require.NoError(t, disabled.Create(ctx))

	got := schema.Egress{ID: disabled.ID}
	require.NoError(t, got.Get(ctx))
	assert.False(t, got.BypassEgressRoutes)

	enabled := &schema.Egress{
		ID:                 uuid.NewString(),
		TenantID:           "tenant-1",
		Name:               "exit-bypass",
		Network:            "net-1",
		Type:               schema.EgressTypeInternet,
		Range:              "*",
		BypassEgressRoutes: true,
		Status:             true,
	}
	require.NoError(t, enabled.Create(ctx))

	got = schema.Egress{ID: enabled.ID}
	require.NoError(t, got.Get(ctx))
	assert.True(t, got.BypassEgressRoutes)
}
