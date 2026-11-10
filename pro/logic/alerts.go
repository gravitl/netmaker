package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"gorm.io/datatypes"
)

var alertHttpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// SendAlertEmail is hooked by pro/initialize.go to avoid circular imports.
var SendAlertEmail = func(ctx context.Context, recipients []string, alert *schema.AlertRecord) error {
	return nil
}

// AlertTriggerRequest carries the parameters needed to evaluate and trigger an alert.
type AlertTriggerRequest struct {
	TenantID    string
	NetworkID   string
	EventType   schema.AlertEventType
	Severity    string // optional override: info, warning, critical
	SubjectID   string
	SubjectName string
	SubjectType schema.SubjectType
	Summary     string
	Details     interface{}
}

// TriggerAlert evaluates matching rules for the given event, checks cooldown, creates AlertRecords and dispatches notifications.
func TriggerAlert(ctx context.Context, req AlertTriggerRequest) error {
	tenantCtx := ctx
	if req.TenantID != "" {
		tenantCtx = scope.WithContext(db.WithContext(ctx), scope.TenantScope, req.TenantID)
	}

	rules, err := (&schema.AlertRule{}).ListActiveByEventAndNetwork(tenantCtx, req.EventType, req.NetworkID)
	if err != nil {
		slog.Error("failed to list active alert rules", "event_type", req.EventType, "tenant_id", req.TenantID, "error", err)
		return err
	}
	if len(rules) == 0 {
		return nil
	}

	detailsJSON, _ := json.Marshal(req.Details)

	for _, rule := range rules {
		cooldownMin := rule.CooldownMinutes
		if cooldownMin <= 0 {
			cooldownMin = 15
		}
		since := time.Now().UTC().Add(-time.Duration(cooldownMin) * time.Minute)

		existing, err := (&schema.AlertRecord{}).FindRecentFiring(tenantCtx, rule.ID, req.SubjectID, since)
		if err == nil && existing != nil {
			slog.Debug("alert in cooldown, suppressing notification",
				"rule_id", rule.ID, "rule_name", rule.Name, "subject_id", req.SubjectID)
			continue
		}

		severity := req.Severity
		if severity == "" {
			severity = rule.Severity
		}
		if severity == "" {
			severity = "warning"
		}

		alert := &schema.AlertRecord{
			ID:          uuid.NewString(),
			TenantID:    rule.TenantID,
			NetworkID:   req.NetworkID,
			RuleID:      rule.ID,
			RuleName:    rule.Name,
			EventType:   req.EventType,
			Severity:    severity,
			SubjectID:   req.SubjectID,
			SubjectName: req.SubjectName,
			SubjectType: req.SubjectType,
			Summary:     req.Summary,
			Details:     datatypes.JSON(detailsJSON),
			Status:      schema.AlertStatusFiring,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		}
		fmt.Println("last")
		if err := alert.Create(tenantCtx); err != nil {
			slog.Error("failed to create alert record", "rule_id", rule.ID, "error", err)
			continue
		}

		slog.Info("network alert fired", "alert_id", alert.ID, "rule_name", alert.RuleName, "summary", alert.Summary)

		// understanding from here.
		// Audit Log & SIEM export
		LogEvent(tenantCtx, &models.Event{
			Action: schema.AlertTriggered,
			Source: models.Subject{
				ID:   rule.ID,
				Name: rule.Name,
				Type: schema.AlertRuleSub,
			},
			Target: models.Subject{
				ID:   alert.ID,
				Name: alert.Summary,
				Type: schema.AlertSub,
			},
			TriggeredBy: "alert_engine",
			NetworkID:   schema.NetworkID(alert.NetworkID),
		})

		// Dispatch notifications
		if len(rule.ChannelIDs) > 0 {
			channels, err := (&schema.NotificationChannel{}).GetByIDs(tenantCtx, rule.ChannelIDs)
			if err != nil {
				slog.Error("failed to get notification channels for alert", "rule_id", rule.ID, "error", err)
				continue
			}
			go dispatchAlertToChannels(tenantCtx, channels, alert)
		}
	}

	return nil
}

// ResolveAlert marks active alerts for the given subject and eventType as resolved.
func ResolveAlert(ctx context.Context, tenantID, networkID, subjectID string, eventType schema.AlertEventType) error {
	tenantCtx := ctx
	if tenantID != "" {
		tenantCtx = scope.WithContext(db.WithContext(ctx), scope.TenantScope, tenantID)
	}

	activeAlerts, err := (&schema.AlertRecord{}).FindActiveFiringBySubject(tenantCtx, eventType, subjectID)
	if err != nil {
		return err
	}

	for _, alert := range activeAlerts {
		if err := alert.Resolve(tenantCtx); err != nil {
			slog.Error("failed to resolve alert", "alert_id", alert.ID, "error", err)
			continue
		}
		slog.Info("network alert resolved", "alert_id", alert.ID, "rule_name", alert.RuleName, "subject_id", subjectID)

		LogEvent(tenantCtx, &models.Event{
			Action: schema.AlertResolved,
			Source: models.Subject{
				ID:   alert.RuleID,
				Name: alert.RuleName,
				Type: schema.AlertRuleSub,
			},
			Target: models.Subject{
				ID:   alert.ID,
				Name: alert.Summary,
				Type: schema.AlertSub,
			},
			TriggeredBy: "alert_engine",
			NetworkID:   schema.NetworkID(alert.NetworkID),
		})
	}

	return nil
}

func dispatchAlertToChannels(ctx context.Context, channels []schema.NotificationChannel, alert *schema.AlertRecord) {
	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}
		if err := DispatchNotification(ctx, &ch, alert); err != nil {
			slog.Error("failed to dispatch notification to channel",
				"channel_id", ch.ID, "channel_name", ch.Name, "channel_type", ch.Type, "error", err)
		}
	}
}

// DispatchNotification sends an alert to a specific notification channel.
func DispatchNotification(ctx context.Context, channel *schema.NotificationChannel, alert *schema.AlertRecord) error {
	switch channel.Type {
	case schema.ChannelEmail:
		return dispatchEmail(ctx, channel, alert)
	case schema.ChannelWebhook:
		return dispatchWebhook(ctx, channel, alert)
	case schema.ChannelSlack:
		return dispatchSlack(ctx, channel, alert)
	case schema.ChannelTeams:
		return dispatchTeams(ctx, channel, alert)
	default:
		return fmt.Errorf("unsupported notification channel type: %s", channel.Type)
	}
}

type emailConfig struct {
	Recipients []string `json:"recipients"`
}

func dispatchEmail(ctx context.Context, channel *schema.NotificationChannel, alert *schema.AlertRecord) error {
	var cfg emailConfig
	if err := json.Unmarshal(channel.Config, &cfg); err != nil {
		return fmt.Errorf("invalid email channel config: %w", err)
	}
	return SendAlertEmail(ctx, cfg.Recipients, alert)
}

type webhookConfig struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

func dispatchWebhook(ctx context.Context, channel *schema.NotificationChannel, alert *schema.AlertRecord) error {
	var cfg webhookConfig
	if err := json.Unmarshal(channel.Config, &cfg); err != nil {
		return fmt.Errorf("invalid webhook config: %w", err)
	}
	if cfg.URL == "" {
		return fmt.Errorf("webhook URL is required")
	}

	payload := map[string]interface{}{
		"alert_id":     alert.ID,
		"rule_id":      alert.RuleID,
		"rule_name":    alert.RuleName,
		"event_type":   alert.EventType,
		"severity":     alert.Severity,
		"network_id":   alert.NetworkID,
		"subject_id":   alert.SubjectID,
		"subject_name": alert.SubjectName,
		"subject_type": alert.SubjectType,
		"summary":      alert.Summary,
		"status":       alert.Status,
		"created_at":   alert.CreatedAt.Format(time.RFC3339),
	}
	if len(alert.Details) > 0 {
		var detailsMap interface{}
		if err := json.Unmarshal(alert.Details, &detailsMap); err == nil {
			payload["details"] = detailsMap
		}
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return postWithRetry(ctx, cfg.URL, cfg.Headers, data)
}

func dispatchSlack(ctx context.Context, channel *schema.NotificationChannel, alert *schema.AlertRecord) error {
	var cfg webhookConfig
	if err := json.Unmarshal(channel.Config, &cfg); err != nil {
		return fmt.Errorf("invalid slack config: %w", err)
	}
	if cfg.URL == "" {
		return fmt.Errorf("slack webhook URL is required")
	}

	color := "#1976D2" // Info blue
	switch strings.ToLower(alert.Severity) {
	case "critical":
		color = "#D32F2F" // Red
	case "warning":
		color = "#FFA000" // Amber
	}
	if alert.Status == schema.AlertStatusResolved {
		color = "#388E3C" // Green
	}

	headline := fmt.Sprintf("[%s] %s", strings.ToUpper(alert.Severity), alert.Summary)
	slackMsg := map[string]interface{}{
		"text": headline,
		"attachments": []map[string]interface{}{
			{
				"color": color,
				"title": fmt.Sprintf("Rule: %s", alert.RuleName),
				"text":  alert.Summary,
				"fields": []map[string]interface{}{
					{"title": "Severity", "value": strings.ToUpper(alert.Severity), "short": true},
					{"title": "Event Type", "value": string(alert.EventType), "short": true},
					{"title": "Network", "value": alert.NetworkID, "short": true},
					{"title": "Subject", "value": fmt.Sprintf("%s (%s)", alert.SubjectName, alert.SubjectID), "short": true},
					{"title": "Status", "value": string(alert.Status), "short": true},
					{"title": "Time (UTC)", "value": alert.CreatedAt.Format(time.RFC3339), "short": true},
				},
			},
		},
	}

	data, err := json.Marshal(slackMsg)
	if err != nil {
		return err
	}

	return postWithRetry(ctx, cfg.URL, cfg.Headers, data)
}

func dispatchTeams(ctx context.Context, channel *schema.NotificationChannel, alert *schema.AlertRecord) error {
	var cfg webhookConfig
	if err := json.Unmarshal(channel.Config, &cfg); err != nil {
		return fmt.Errorf("invalid teams config: %w", err)
	}
	if cfg.URL == "" {
		return fmt.Errorf("teams webhook URL is required")
	}

	themeColor := "1976D2"
	switch strings.ToLower(alert.Severity) {
	case "critical":
		themeColor = "D32F2F"
	case "warning":
		themeColor = "FFA000"
	}
	if alert.Status == schema.AlertStatusResolved {
		themeColor = "388E3C"
	}

	teamsCard := map[string]interface{}{
		"@type":      "MessageCard",
		"@context":   "http://schema.org/extensions",
		"themeColor": themeColor,
		"summary":    alert.Summary,
		"sections": []map[string]interface{}{
			{
				"activityTitle":    fmt.Sprintf("[%s] %s", strings.ToUpper(alert.Severity), alert.Summary),
				"activitySubtitle": fmt.Sprintf("Rule: %s | Network: %s", alert.RuleName, alert.NetworkID),
				"facts": []map[string]string{
					{"name": "Event Type", "value": string(alert.EventType)},
					{"name": "Severity", "value": strings.ToUpper(alert.Severity)},
					{"name": "Subject", "value": fmt.Sprintf("%s (%s)", alert.SubjectName, alert.SubjectID)},
					{"name": "Status", "value": string(alert.Status)},
					{"name": "Time", "value": alert.CreatedAt.Format(time.RFC3339)},
				},
				"markdown": true,
			},
		},
	}

	data, err := json.Marshal(teamsCard)
	if err != nil {
		return err
	}

	return postWithRetry(ctx, cfg.URL, cfg.Headers, data)
}

func postWithRetry(ctx context.Context, url string, headers map[string]string, data []byte) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(data))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := alertHttpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		lastErr = fmt.Errorf("webhook responded with status: %d", resp.StatusCode)
	}
	return lastErr
}

// TestNotificationChannel sends a synthetic test alert to verify the channel configuration.
func TestNotificationChannel(ctx context.Context, channel *schema.NotificationChannel) error {
	testAlert := &schema.AlertRecord{
		ID:          uuid.NewString(),
		TenantID:    channel.TenantID,
		NetworkID:   "test-network",
		RuleID:      "test-rule",
		RuleName:    "Test Alert Rule",
		EventType:   schema.EventNodeOffline,
		Severity:    "info",
		SubjectID:   "test-node-id",
		SubjectName: "test-node",
		SubjectType: schema.NodeSub,
		Summary:     fmt.Sprintf("This is a test alert from Netmaker for channel '%s'", channel.Name),
		Status:      schema.AlertStatusFiring,
		CreatedAt:   time.Now().UTC(),
	}
	return DispatchNotification(ctx, channel, testAlert)
}

// OnNodeOffline is called when a node misses checkin and is marked offline.
func OnNodeOffline(ctx context.Context, node *schema.Node) {
	if node == nil {
		return
	}
	subjectName := node.ID
	if node.Address != "" {
		subjectName = fmt.Sprintf("Node %s (%s)", node.ID, node.Address)
	}

	req := AlertTriggerRequest{
		TenantID:    node.TenantID,
		NetworkID:   string(node.NetworkID),
		EventType:   schema.EventNodeOffline,
		Severity:    "critical",
		SubjectID:   node.ID,
		SubjectName: subjectName,
		SubjectType: schema.NodeSub,
		Summary:     fmt.Sprintf("%s is offline (missed check-in)", subjectName),
		Details: map[string]interface{}{
			"node_id":       node.ID,
			"host_id":       node.HostID,
			"network_id":    node.NetworkID,
			"last_check_in": node.LastCheckIn,
			"address":       node.Address,
			"is_gateway":    node.IsGateway,
		},
	}
	if node.IsGateway {
		req.Summary = fmt.Sprintf("Gateway %s is offline", subjectName)
		_ = TriggerAlert(ctx, AlertTriggerRequest{
			TenantID:    node.TenantID,
			NetworkID:   string(node.NetworkID),
			EventType:   schema.EventGatewayDegraded,
			Severity:    "critical",
			SubjectID:   node.ID,
			SubjectName: subjectName,
			SubjectType: schema.GatewaySub,
			Summary:     fmt.Sprintf("Gateway %s is offline", subjectName),
			Details:     req.Details,
		})
	}
	_ = TriggerAlert(ctx, req)
}

// OnNodeOnline is called when an offline node checks in and recovers.
func OnNodeOnline(ctx context.Context, node *schema.Node) {
	if node == nil {
		return
	}
	subjectName := node.ID
	if node.Address != "" {
		subjectName = fmt.Sprintf("Node %s (%s)", node.ID, node.Address)
	}

	_ = ResolveAlert(ctx, node.TenantID, string(node.NetworkID), node.ID, schema.EventNodeOffline)
	if node.IsGateway {
		_ = ResolveAlert(ctx, node.TenantID, string(node.NetworkID), node.ID, schema.EventGatewayDegraded)
	}
	_ = TriggerAlert(ctx, AlertTriggerRequest{
		TenantID:    node.TenantID,
		NetworkID:   string(node.NetworkID),
		EventType:   schema.EventNodeRecovered,
		Severity:    "info",
		SubjectID:   node.ID,
		SubjectName: subjectName,
		SubjectType: schema.NodeSub,
		Summary:     fmt.Sprintf("%s has recovered and is back online", subjectName),
		Details: map[string]interface{}{
			"node_id":    node.ID,
			"network_id": node.NetworkID,
		},
	})
}

// EvaluateAlertsForEvent evaluates an incoming audit event against alert rules.
func EvaluateAlertsForEvent(ctx context.Context, e models.Event, a schema.Event) {
	switch e.Action {
	case schema.PostureCheckFailed:
		_ = TriggerAlert(ctx, AlertTriggerRequest{
			TenantID:    e.TenantID,
			NetworkID:   string(e.NetworkID),
			EventType:   schema.EventSecurityViolation,
			Severity:    "critical",
			SubjectID:   e.Source.ID,
			SubjectName: e.Source.Name,
			SubjectType: e.Source.Type,
			Summary:     fmt.Sprintf("Security posture check failed for %s '%s'", e.Source.Type, e.Source.Name),
			Details:     e.Source.Info,
		})
	case schema.DisableAclPolicy:
		_ = TriggerAlert(ctx, AlertTriggerRequest{
			TenantID:    e.TenantID,
			NetworkID:   string(e.NetworkID),
			EventType:   schema.EventAclViolation,
			Severity:    "warning",
			SubjectID:   e.Source.ID,
			SubjectName: e.Source.Name,
			SubjectType: schema.AclSub,
			Summary:     fmt.Sprintf("ACL Policy '%s' was disabled by %s", e.Source.Name, e.TriggeredBy),
			Details:     e.Diff,
		})
	}
}
