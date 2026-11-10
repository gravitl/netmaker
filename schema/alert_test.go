package schema_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestAlertRule_CRUD(t *testing.T) {
	ctx := setupSchemaTest(t)

	rule := &schema.AlertRule{
		TenantID:        "tenant-1",
		NetworkID:       "net-1",
		Name:            "Node Offline Alert",
		EventType:       schema.EventNodeOffline,
		Severity:        "critical",
		Threshold:       60,
		Enabled:         true,
		CooldownMinutes: 15,
		ChannelIDs:      datatypes.JSONSlice[string]{"ch-1", "ch-2"},
	}

	require.NoError(t, rule.Create(ctx))
	assert.NotEmpty(t, rule.ID)

	// Get
	fetched := &schema.AlertRule{ID: rule.ID}
	require.NoError(t, fetched.Get(ctx))
	assert.Equal(t, "Node Offline Alert", fetched.Name)
	assert.Equal(t, schema.EventNodeOffline, fetched.EventType)
	assert.Equal(t, "critical", fetched.Severity)
	assert.True(t, fetched.Enabled)
	assert.Equal(t, 2, len(fetched.ChannelIDs))

	// Update
	fetched.Name = "Updated Node Offline Alert"
	fetched.Enabled = false
	require.NoError(t, fetched.Update(ctx))

	refetched := &schema.AlertRule{ID: rule.ID}
	require.NoError(t, refetched.Get(ctx))
	assert.Equal(t, "Updated Node Offline Alert", refetched.Name)

	// List
	rules, err := (&schema.AlertRule{}).ListByTenant(ctx, "net-1")
	require.NoError(t, err)
	assert.NotEmpty(t, rules)

	// Delete
	require.NoError(t, rule.Delete(ctx))
	assert.Error(t, (&schema.AlertRule{ID: rule.ID}).Get(ctx))
}

func TestNotificationChannel_CRUD(t *testing.T) {
	ctx := setupSchemaTest(t)

	cfg, _ := json.Marshal(map[string]interface{}{
		"url": "https://hooks.slack.com/services/test/test/test",
	})

	ch := &schema.NotificationChannel{
		TenantID: "tenant-1",
		Name:     "DevOps Slack",
		Type:     schema.ChannelSlack,
		Config:   datatypes.JSON(cfg),
		Enabled:  true,
	}

	require.NoError(t, ch.Create(ctx))
	assert.NotEmpty(t, ch.ID)

	// Get
	fetched := &schema.NotificationChannel{ID: ch.ID}
	require.NoError(t, fetched.Get(ctx))
	assert.Equal(t, "DevOps Slack", fetched.Name)
	assert.Equal(t, schema.ChannelSlack, fetched.Type)

	// Update
	fetched.Name = "Production Slack"
	require.NoError(t, fetched.Update(ctx))

	refetched := &schema.NotificationChannel{ID: ch.ID}
	require.NoError(t, refetched.Get(ctx))
	assert.Equal(t, "Production Slack", refetched.Name)

	// List
	channels, err := (&schema.NotificationChannel{}).ListByTenant(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, channels)

	// GetByIDs
	byIDs, err := (&schema.NotificationChannel{}).GetByIDs(ctx, []string{ch.ID})
	require.NoError(t, err)
	assert.Len(t, byIDs, 1)

	// Delete
	require.NoError(t, ch.Delete(ctx))
	assert.Error(t, (&schema.NotificationChannel{ID: ch.ID}).Get(ctx))
}

func TestAlertRecord_Lifecycle(t *testing.T) {
	ctx := setupSchemaTest(t)

	details, _ := json.Marshal(map[string]interface{}{
		"host_id": "host-1",
		"ip":      "10.0.0.1",
	})

	record := &schema.AlertRecord{
		TenantID:    "tenant-1",
		NetworkID:   "net-1",
		RuleID:      "rule-1",
		RuleName:    "Node Offline Alert",
		EventType:   schema.EventNodeOffline,
		Severity:    "critical",
		SubjectID:   uuid.NewString(),
		SubjectName: "node-1",
		SubjectType: schema.NodeSub,
		Summary:     "Node node-1 went offline",
		Details:     datatypes.JSON(details),
		Status:      schema.AlertStatusFiring,
	}

	require.NoError(t, record.Create(ctx))
	assert.NotEmpty(t, record.ID)

	// FindRecentFiring
	recent, err := (&schema.AlertRecord{}).FindRecentFiring(ctx, "rule-1", record.SubjectID, time.Now().UTC().Add(-time.Hour))
	require.NoError(t, err)
	assert.NotNil(t, recent)
	assert.Equal(t, record.ID, recent.ID)

	// Acknowledge
	require.NoError(t, record.Acknowledge(ctx, "admin@example.com"))
	ackRecord := &schema.AlertRecord{ID: record.ID}
	require.NoError(t, ackRecord.Get(ctx))
	assert.Equal(t, schema.AlertStatusAcknowledged, ackRecord.Status)
	assert.Equal(t, "admin@example.com", ackRecord.AcknowledgedBy)
	assert.NotNil(t, ackRecord.AcknowledgedAt)

	// Resolve
	require.NoError(t, record.Resolve(ctx))
	resRecord := &schema.AlertRecord{ID: record.ID}
	require.NoError(t, resRecord.Get(ctx))
	assert.Equal(t, schema.AlertStatusResolved, resRecord.Status)
	assert.NotNil(t, resRecord.ResolvedAt)

	// List
	list, err := (&schema.AlertRecord{}).List(ctx, schema.AlertStatusResolved, "net-1", "critical", time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.NotEmpty(t, list)
}
