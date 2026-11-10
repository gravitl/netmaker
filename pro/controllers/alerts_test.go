package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func setupAlertControllerTest(t *testing.T) (context.Context, *mux.Router) {
	t.Helper()
	t.Chdir(t.TempDir())
	require.NoError(t, db.InitializeDB(schema.ListModels()...))
	t.Cleanup(db.CloseDB)

	ctx := scope.WithContext(db.WithContext(context.Background()), scope.TenantScope, "tenant-1")

	r := mux.NewRouter()
	AlertHandlers(r)
	return ctx, r
}

func TestAlertRulesController_CRUD(t *testing.T) {
	ctx, _ := setupAlertControllerTest(t)

	// 1. Create Rule
	ruleReq := schema.AlertRule{
		TenantID:        "tenant-1",
		NetworkID:       "net-1",
		Name:            "Node Downtime Rule",
		EventType:       schema.EventNodeOffline,
		Severity:        "critical",
		Threshold:       10,
		Enabled:         true,
		CooldownMinutes: 15,
		ChannelIDs:      datatypes.JSONSlice[string]{"ch-1"},
	}
	body, _ := json.Marshal(ruleReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/rules", bytes.NewBuffer(body))
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	createAlertRule(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var createResp models.SuccessResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&createResp))
	var createdRule schema.AlertRule
	respBytes, _ := json.Marshal(createResp.Response)
	_ = json.Unmarshal(respBytes, &createdRule)
	assert.NotEmpty(t, createdRule.ID)
	assert.Equal(t, "Node Downtime Rule", createdRule.Name)

	// 2. Get Rule
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/alerts/rules/"+createdRule.ID, nil)
	reqGet = reqGet.WithContext(ctx)
	reqGet = mux.SetURLVars(reqGet, map[string]string{"id": createdRule.ID})
	wGet := httptest.NewRecorder()
	getAlertRule(wGet, reqGet)
	assert.Equal(t, http.StatusOK, wGet.Code)

	// 3. Update Rule
	updateReq := schema.AlertRule{
		Name:     "Node Downtime Rule Renamed",
		Severity: "warning",
		Enabled:  false,
	}
	updBody, _ := json.Marshal(updateReq)
	reqUpd := httptest.NewRequest(http.MethodPut, "/api/v1/alerts/rules/"+createdRule.ID, bytes.NewBuffer(updBody))
	reqUpd = reqUpd.WithContext(ctx)
	reqUpd = mux.SetURLVars(reqUpd, map[string]string{"id": createdRule.ID})
	wUpd := httptest.NewRecorder()
	updateAlertRule(wUpd, reqUpd)
	assert.Equal(t, http.StatusOK, wUpd.Code)

	// 4. List Rules
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/alerts/rules?network_id=net-1", nil)
	reqList = reqList.WithContext(ctx)
	wList := httptest.NewRecorder()
	listAlertRules(wList, reqList)
	assert.Equal(t, http.StatusOK, wList.Code)

	// 5. Delete Rule
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/alerts/rules/"+createdRule.ID, nil)
	reqDel = reqDel.WithContext(ctx)
	reqDel = mux.SetURLVars(reqDel, map[string]string{"id": createdRule.ID})
	wDel := httptest.NewRecorder()
	deleteAlertRule(wDel, reqDel)
	assert.Equal(t, http.StatusOK, wDel.Code)
}

func TestNotificationChannelsController_CRUD(t *testing.T) {
	ctx, _ := setupAlertControllerTest(t)

	cfg, _ := json.Marshal(map[string]interface{}{
		"url": "https://hooks.slack.com/services/test",
	})
	chReq := schema.NotificationChannel{
		TenantID: "tenant-1",
		Name:     "Slack Alert Channel",
		Type:     schema.ChannelSlack,
		Config:   datatypes.JSON(cfg),
		Enabled:  true,
	}
	body, _ := json.Marshal(chReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/channels", bytes.NewBuffer(body))
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	createNotificationChannel(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var createResp models.SuccessResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&createResp))
	var createdCh schema.NotificationChannel
	respBytes, _ := json.Marshal(createResp.Response)
	_ = json.Unmarshal(respBytes, &createdCh)
	assert.NotEmpty(t, createdCh.ID)

	// Get Channel
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/alerts/channels/"+createdCh.ID, nil)
	reqGet = reqGet.WithContext(ctx)
	reqGet = mux.SetURLVars(reqGet, map[string]string{"id": createdCh.ID})
	wGet := httptest.NewRecorder()
	getNotificationChannel(wGet, reqGet)
	assert.Equal(t, http.StatusOK, wGet.Code)

	// List Channels
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/alerts/channels", nil)
	reqList = reqList.WithContext(ctx)
	wList := httptest.NewRecorder()
	listNotificationChannels(wList, reqList)
	assert.Equal(t, http.StatusOK, wList.Code)

	// Delete Channel
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/alerts/channels/"+createdCh.ID, nil)
	reqDel = reqDel.WithContext(ctx)
	reqDel = mux.SetURLVars(reqDel, map[string]string{"id": createdCh.ID})
	wDel := httptest.NewRecorder()
	deleteNotificationChannel(wDel, reqDel)
	assert.Equal(t, http.StatusOK, wDel.Code)
}

func TestAlertHistoryAndAckController(t *testing.T) {
	ctx, _ := setupAlertControllerTest(t)

	// Create an alert record
	record := &schema.AlertRecord{
		TenantID:    "tenant-1",
		NetworkID:   "net-1",
		RuleID:      "rule-1",
		RuleName:    "Node Offline Alert",
		EventType:   schema.EventNodeOffline,
		Severity:    "critical",
		SubjectID:   uuid.NewString(),
		SubjectName: "db-node-1",
		SubjectType: schema.NodeSub,
		Summary:     "Node db-node-1 went offline",
		Status:      schema.AlertStatusFiring,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	require.NoError(t, record.Create(ctx))

	// List History
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/alerts/history?status=firing&network_id=net-1", nil)
	reqList = reqList.WithContext(ctx)
	wList := httptest.NewRecorder()
	listAlertHistory(wList, reqList)
	assert.Equal(t, http.StatusOK, wList.Code)

	// Acknowledge Alert
	ackBody, _ := json.Marshal(ackRequest{AcknowledgedBy: "oncall-eng@netmaker.io"})
	reqAck := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/"+record.ID+"/ack", bytes.NewBuffer(ackBody))
	reqAck = reqAck.WithContext(ctx)
	reqAck = mux.SetURLVars(reqAck, map[string]string{"id": record.ID})
	wAck := httptest.NewRecorder()
	acknowledgeAlert(wAck, reqAck)
	assert.Equal(t, http.StatusOK, wAck.Code)

	refetched := &schema.AlertRecord{ID: record.ID}
	require.NoError(t, refetched.Get(ctx))
	assert.Equal(t, schema.AlertStatusAcknowledged, refetched.Status)
	assert.Equal(t, "oncall-eng@netmaker.io", refetched.AcknowledgedBy)

	// Resolve Alert
	reqResolve := httptest.NewRequest(http.MethodPost, "/api/v1/alerts/"+record.ID+"/resolve", nil)
	reqResolve = reqResolve.WithContext(ctx)
	reqResolve = mux.SetURLVars(reqResolve, map[string]string{"id": record.ID})
	wResolve := httptest.NewRecorder()
	resolveAlertManual(wResolve, reqResolve)
	assert.Equal(t, http.StatusOK, wResolve.Code)

	require.NoError(t, refetched.Get(ctx))
	assert.Equal(t, schema.AlertStatusResolved, refetched.Status)
}

