package schema_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/schema"
	testutils "github.com/gravitl/netmaker/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNode_UpsertViolations(t *testing.T) {
	ctx := setupSchemaTest(t)
	network := testutils.CreateIPv4Network(t, ctx, "net-1")
	host := testutils.CreateHost(t, ctx, "host-1")

	node := &schema.Node{
		ID:        uuid.NewString(),
		TenantID:  "tenant-1",
		HostID:    host.ID.String(),
		NetworkID: network.ID,
	}
	require.NoError(t, node.Create(ctx))

	node.PostureCheckSeverity = schema.SeverityHigh
	node.PostureCheckLastEvaluationCycleID = uuid.NewString()
	node.PostureCheckLastEvaluatedAt = time.Now().UTC()
	require.NoError(t, node.UpsertViolations(ctx, []schema.PostureCheckViolation{
		{CheckID: "check-1", Name: "os", Severity: schema.SeverityHigh},
	}))

	violations, err := node.ListViolations(ctx)
	require.NoError(t, err)
	require.Len(t, violations, 1)
	assert.Equal(t, schema.PostureCheckSubjectTypeDevice, violations[0].SubjectType)
	assert.Equal(t, node.ID, violations[0].SubjectID)
	assert.Equal(t, "tenant-1", violations[0].TenantID)
}
