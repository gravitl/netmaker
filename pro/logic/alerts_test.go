package logic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func setupAlertTest(t *testing.T) context.Context {
	t.Helper()
	t.Chdir(t.TempDir())
	require.NoError(t, db.InitializeDB(schema.ListModels()...))
	t.Cleanup(db.CloseDB)

	return scope.WithContext(db.WithContext(context.Background()), scope.TenantScope, "tenant-1")
}

func TestTriggerAlert_AndCooldown(t *testing.T) {
	ctx := setupAlertTest(t)

	rule := &schema.AlertRule{
		TenantID:        "tenant-1",
		NetworkID:       "net-1",
		Name:            "Node Offline Critical",
		EventType:       schema.EventNodeOffline,
		Severity:        "critical",
		Enabled:         true,
		CooldownMinutes: 30,
	}
	require.NoError(t, rule.Create(ctx))

	nodeID := uuid.NewString()

	// First trigger -> should create firing alert
	err := TriggerAlert(ctx, AlertTriggerRequest{
		TenantID:    "tenant-1",
		NetworkID:   "net-1",
		EventType:   schema.EventNodeOffline,
		SubjectID:   nodeID,
		SubjectName: "web-prod-1",
		SubjectType: schema.NodeSub,
		Summary:     "Node web-prod-1 is offline",
	})
	require.NoError(t, err)

	records, err := (&schema.AlertRecord{}).List(ctx, schema.AlertStatusFiring, "net-1", "critical", time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, rule.ID, records[0].RuleID)
	assert.Equal(t, "Node Offline Critical", records[0].RuleName)
	assert.Equal(t, nodeID, records[0].SubjectID)
	assert.Equal(t, schema.AlertStatusFiring, records[0].Status)

	// Second trigger within cooldown -> should debounce/skip
	err = TriggerAlert(ctx, AlertTriggerRequest{
		TenantID:    "tenant-1",
		NetworkID:   "net-1",
		EventType:   schema.EventNodeOffline,
		SubjectID:   nodeID,
		SubjectName: "web-prod-1",
		SubjectType: schema.NodeSub,
		Summary:     "Node web-prod-1 is offline again",
	})
	require.NoError(t, err)

	recordsAfter, err := (&schema.AlertRecord{}).List(ctx, schema.AlertStatusFiring, "net-1", "critical", time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Len(t, recordsAfter, 1, "alert in cooldown must not duplicate records")
}

func TestTriggerAlert_ChannelsDispatch(t *testing.T) {
	ctx := setupAlertTest(t)

	var webhookCalled, slackCalled, teamsCalled int32
	var lastSlackPayload map[string]interface{}
	var lastTeamsPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch path {
		case "/webhook":
			atomic.AddInt32(&webhookCalled, 1)
			assert.Equal(t, "Bearer secret-token", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
		case "/slack":
			atomic.AddInt32(&slackCalled, 1)
			_ = json.NewDecoder(r.Body).Decode(&lastSlackPayload)
			w.WriteHeader(http.StatusOK)
		case "/teams":
			atomic.AddInt32(&teamsCalled, 1)
			_ = json.NewDecoder(r.Body).Decode(&lastTeamsPayload)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Webhook channel
	whCfg, _ := json.Marshal(map[string]interface{}{
		"url": server.URL + "/webhook",
		"headers": map[string]string{
			"Authorization": "Bearer secret-token",
		},
	})
	chWebhook := &schema.NotificationChannel{
		TenantID: "tenant-1",
		Name:     "Ops Webhook",
		Type:     schema.ChannelWebhook,
		Config:   datatypes.JSON(whCfg),
		Enabled:  true,
	}
	require.NoError(t, chWebhook.Create(ctx))

	// Slack channel
	slackCfg, _ := json.Marshal(map[string]interface{}{
		"url": server.URL + "/slack",
	})
	chSlack := &schema.NotificationChannel{
		TenantID: "tenant-1",
		Name:     "DevOps Slack",
		Type:     schema.ChannelSlack,
		Config:   datatypes.JSON(slackCfg),
		Enabled:  true,
	}
	require.NoError(t, chSlack.Create(ctx))

	// Teams channel
	teamsCfg, _ := json.Marshal(map[string]interface{}{
		"url": server.URL + "/teams",
	})
	chTeams := &schema.NotificationChannel{
		TenantID: "tenant-1",
		Name:     "DevOps Teams",
		Type:     schema.ChannelTeams,
		Config:   datatypes.JSON(teamsCfg),
		Enabled:  true,
	}
	require.NoError(t, chTeams.Create(ctx))

	rule := &schema.AlertRule{
		TenantID:        "tenant-1",
		NetworkID:       "net-1",
		Name:            "Gateway Down Alert",
		EventType:       schema.EventGatewayDegraded,
		Severity:        "critical",
		Enabled:         true,
		CooldownMinutes: 1,
		ChannelIDs:      datatypes.JSONSlice[string]{chWebhook.ID, chSlack.ID, chTeams.ID},
	}
	require.NoError(t, rule.Create(ctx))

	err := TriggerAlert(ctx, AlertTriggerRequest{
		TenantID:    "tenant-1",
		NetworkID:   "net-1",
		EventType:   schema.EventGatewayDegraded,
		SubjectID:   "gw-1",
		SubjectName: "gateway-us-east",
		SubjectType: schema.GatewaySub,
		Summary:     "Gateway gateway-us-east is degraded",
	})
	require.NoError(t, err)

	// Wait briefly for async dispatches
	time.Sleep(300 * time.Millisecond)

	assert.Equal(t, int32(1), atomic.LoadInt32(&webhookCalled))
	assert.Equal(t, int32(1), atomic.LoadInt32(&slackCalled))
	assert.Equal(t, int32(1), atomic.LoadInt32(&teamsCalled))

	assert.Contains(t, lastSlackPayload["text"], "CRITICAL")
	assert.Equal(t, "MessageCard", lastTeamsPayload["@type"])
}

func TestResolveAlert(t *testing.T) {
	ctx := setupAlertTest(t)

	rule := &schema.AlertRule{
		TenantID:  "tenant-1",
		NetworkID: "net-1",
		Name:      "Node Offline Alert",
		EventType: schema.EventNodeOffline,
		Severity:  "critical",
		Enabled:   true,
	}
	require.NoError(t, rule.Create(ctx))

	nodeID := uuid.NewString()

	err := TriggerAlert(ctx, AlertTriggerRequest{
		TenantID:    "tenant-1",
		NetworkID:   "net-1",
		EventType:   schema.EventNodeOffline,
		SubjectID:   nodeID,
		SubjectName: "node-xyz",
		SubjectType: schema.NodeSub,
		Summary:     "Node node-xyz offline",
	})
	require.NoError(t, err)

	// Resolve
	err = ResolveAlert(ctx, "tenant-1", "net-1", nodeID, schema.EventNodeOffline)
	require.NoError(t, err)

	records, err := (&schema.AlertRecord{}).List(ctx, schema.AlertStatusResolved, "net-1", "", time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, schema.AlertStatusResolved, records[0].Status)
	assert.NotNil(t, records[0].ResolvedAt)
}

func TestOnNodeOfflineAndOnline(t *testing.T) {
	ctx := setupAlertTest(t)

	ruleOffline := &schema.AlertRule{
		TenantID:  "tenant-1",
		NetworkID: "net-1",
		Name:      "Node Offline",
		EventType: schema.EventNodeOffline,
		Severity:  "critical",
		Enabled:   true,
	}
	require.NoError(t, ruleOffline.Create(ctx))

	ruleRecovered := &schema.AlertRule{
		TenantID:  "tenant-1",
		NetworkID: "net-1",
		Name:      "Node Recovered",
		EventType: schema.EventNodeRecovered,
		Severity:  "info",
		Enabled:   true,
	}
	require.NoError(t, ruleRecovered.Create(ctx))

	node := &schema.Node{
		ID:        uuid.NewString(),
		TenantID:  "tenant-1",
		NetworkID: "net-1",
		Address:   "10.0.0.5",
		Status:    schema.OfflineSt,
	}

	OnNodeOffline(ctx, node)

	firing, err := (&schema.AlertRecord{}).List(ctx, schema.AlertStatusFiring, "net-1", "critical", time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, firing, 1)
	assert.Contains(t, firing[0].Summary, "offline")

	// Now node comes online
	OnNodeOnline(ctx, node)

	resolved, err := (&schema.AlertRecord{}).List(ctx, schema.AlertStatusResolved, "net-1", "", time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, resolved, 1)

	recovered, err := (&schema.AlertRecord{}).List(ctx, schema.AlertStatusFiring, "net-1", "info", time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, recovered, 1)
	assert.Equal(t, schema.EventNodeRecovered, recovered[0].EventType)
}

func TestEvaluateAlertsForEvent(t *testing.T) {
	ctx := setupAlertTest(t)

	ruleSecurity := &schema.AlertRule{
		TenantID:  "tenant-1",
		NetworkID: "net-1",
		Name:      "Security Posture Violation",
		EventType: schema.EventSecurityViolation,
		Severity:  "critical",
		Enabled:   true,
	}
	require.NoError(t, ruleSecurity.Create(ctx))

	event := models.Event{
		TenantID:  "tenant-1",
		NetworkID: "net-1",
		Action:    schema.PostureCheckFailed,
		Source: models.Subject{
			ID:   "device-101",
			Name: "laptop-alice",
			Type: schema.DeviceSub,
		},
		TriggeredBy: "posture_engine",
	}

	EvaluateAlertsForEvent(ctx, event, schema.Event{})

	records, err := (&schema.AlertRecord{}).List(ctx, schema.AlertStatusFiring, "net-1", "critical", time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, schema.EventSecurityViolation, records[0].EventType)
	assert.Contains(t, records[0].Summary, "posture check failed")
}

