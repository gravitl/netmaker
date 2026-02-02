package email

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// JITExpiredMail - mail for notifying users when their JIT grant expires or is revoked
type JITExpiredMail struct {
	BodyBuilder EmailBodyBuilder
	Grant       *schema.JITGrant
	Request     *schema.JITRequest
	Network     models.Network
	IsRevoked   bool
}

// SendJITExpirationEmail - sends email notification to user when JIT grant expires
func SendJITExpirationEmail(grant *schema.JITGrant, request *schema.JITRequest, network models.Network, isRevoked bool) error {
	mail := JITExpiredMail{
		BodyBuilder: &EmailBodyBuilderWithH1HeadlineAndImage{},
		Grant:       grant,
		Request:     request,
		Network:     network,
		IsRevoked:   isRevoked,
	}
	// Skip sending email if username is not a valid email address
	if !IsValid(request.UserName) {
		slog.Warn("skipping JIT expiration email with non-email username", "user", request.UserName)
		return nil
	}
	notification := Notification{
		RecipientMail: request.UserName,
		RecipientName: request.UserName,
	}

	return GetClient().SendEmail(context.Background(), notification, mail)
}

// GetSubject - gets the subject of the email
func (mail JITExpiredMail) GetSubject(info Notification) string {
	if mail.IsRevoked {
		return fmt.Sprintf("JIT Access Revoked: %s", mail.Network.NetID)
	}
	return fmt.Sprintf("JIT Access Expired: %s", mail.Network.NetID)
}

// GetBody - gets the body of the email
func (mail JITExpiredMail) GetBody(info Notification) string {
	var headline, message string
	if mail.IsRevoked {
		headline = "JIT Access Revoked"
		message = fmt.Sprintf("Your Just-In-Time access to network <strong>%s</strong> has been revoked by an administrator.", mail.Network.NetID)
	} else {
		headline = "JIT Access Expired"
		message = fmt.Sprintf("Your Just-In-Time access to network <strong>%s</strong> has expired.", mail.Network.NetID)
	}

	content := mail.BodyBuilder.
		WithHeadline(headline).
		WithParagraph(message).
		WithParagraph("Access Details:").
		WithHtml("<ul>").
		WithHtml(fmt.Sprintf("<li><strong>Network:</strong> %s</li>", mail.Network.NetID)).
		WithHtml(fmt.Sprintf("<li><strong>Granted At:</strong> %s</li>", formatUTCTime(mail.Grant.GrantedAt))).
		WithHtml(fmt.Sprintf("<li><strong>Expired At:</strong> %s</li>", formatUTCTime(mail.Grant.ExpiresAt))).
		WithHtml("</ul>").
		WithParagraph("Your access to this network has been terminated. If you need access again, please submit a new JIT access request.").
		WithParagraph("Best Regards,").
		WithParagraph("The Netmaker Team").
		Build()

	return content
}

// formatUTCTime - formats a time in UTC to a clean, readable format
func formatUTCTime(t time.Time) string {
	utcTime := t.UTC()
	return utcTime.Format("January 2, 2006 at 3:04 PM UTC")
}
