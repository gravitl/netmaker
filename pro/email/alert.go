package email

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gravitl/netmaker/schema"
)

// AlertEmailMail implements Mail for formatting operator email alerts.
type AlertEmailMail struct {
	Alert *schema.AlertRecord
}

func (m AlertEmailMail) GetSubject(info Notification) string {
	return fmt.Sprintf("[%s] Network Alert: %s", strings.ToUpper(m.Alert.Severity), m.Alert.Summary)
}

func (m AlertEmailMail) GetBody(info Notification) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; color: #333; line-height: 1.5; padding: 20px; }
    .card { border: 1px solid #e0e0e0; border-radius: 8px; padding: 24px; max-width: 600px; box-shadow: 0 2px 4px rgba(0,0,0,0.05); }
    .badge { display: inline-block; padding: 4px 10px; border-radius: 4px; font-weight: bold; font-size: 12px; text-transform: uppercase; color: #fff; }
    .badge-critical { background-color: #d32f2f; }
    .badge-warning { background-color: #f57c00; }
    .badge-info { background-color: #1976d2; }
    .meta { margin-top: 16px; border-collapse: collapse; width: 100%%; }
    .meta td { padding: 8px 0; border-bottom: 1px solid #f0f0f0; }
    .meta td.label { font-weight: 600; width: 120px; color: #666; }
  </style>
</head>
<body>
  <div class="card">
    <span class="badge badge-%s">%s</span>
    <h2 style="margin: 12px 0;">%s</h2>
    <table class="meta">
      <tr><td class="label">Rule</td><td>%s</td></tr>
      <tr><td class="label">Event Type</td><td>%s</td></tr>
      <tr><td class="label">Network ID</td><td>%s</td></tr>
      <tr><td class="label">Subject</td><td>%s (%s)</td></tr>
      <tr><td class="label">Time (UTC)</td><td>%s</td></tr>
    </table>
  </div>
</body>
</html>`,
		strings.ToLower(m.Alert.Severity),
		strings.ToUpper(m.Alert.Severity),
		m.Alert.Summary,
		m.Alert.RuleName,
		m.Alert.EventType,
		m.Alert.NetworkID,
		m.Alert.SubjectName,
		m.Alert.SubjectID,
		m.Alert.CreatedAt.Format(time.RFC3339),
	)
}

// SendAlertEmail sends an alert email to all configured recipient addresses.
func SendAlertEmail(ctx context.Context, recipients []string, alert *schema.AlertRecord) error {
	mail := AlertEmailMail{Alert: alert}
	var lastErr error
	for _, recipient := range recipients {
		recipient = strings.TrimSpace(recipient)
		if recipient == "" || !IsValid(recipient) {
			continue
		}
		notif := Notification{
			RecipientMail: recipient,
			RecipientName: "Operator",
			ProductName:   "Netmaker",
		}
		if err := GetClient().SendEmail(ctx, notif, mail); err != nil {
			lastErr = err
			slog.Error("failed to send alert email", "recipient", recipient, "error", err)
		}
	}
	return lastErr
}

