package email

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// JITDeniedMail - mail for notifying users when their JIT request is denied
type JITDeniedMail struct {
	BodyBuilder EmailBodyBuilder
	Request     *schema.JITRequest
	Network     models.Network
}

// SendJITDeniedEmail - sends email notification to user when JIT request is denied
func SendJITDeniedEmail(request *schema.JITRequest, network models.Network) error {
	mail := JITDeniedMail{
		BodyBuilder: &EmailBodyBuilderWithH1HeadlineAndImage{},
		Request:     request,
		Network:     network,
	}
	// Skip sending email if username is not a valid email address
	if !IsValid(request.UserName) {
		slog.Warn("skipping JIT denied email with non-email username", "user", request.UserName)
		return nil
	}
	notification := Notification{
		RecipientMail: request.UserName,
		RecipientName: request.UserName,
	}

	return GetClient().SendEmail(context.Background(), notification, mail)
}

// GetSubject - gets the subject of the email
func (mail JITDeniedMail) GetSubject(info Notification) string {
	return fmt.Sprintf("JIT Access Request Denied: %s", mail.Network.NetID)
}

// GetBody - gets the body of the email
func (mail JITDeniedMail) GetBody(info Notification) string {
	content := mail.BodyBuilder.
		WithHeadline("JIT Access Request Denied").
		WithParagraph(fmt.Sprintf("Your request for Just-In-Time access to network <strong>%s</strong> has been denied.", mail.Network.NetID)).
		WithParagraph("Request Details:").
		WithHtml("<ul>").
		WithHtml(fmt.Sprintf("<li><strong>Network:</strong> %s</li>", mail.Network.NetID)).
		WithHtml(fmt.Sprintf("<li><strong>Requested At:</strong> %s</li>", formatUTCTime(mail.Request.RequestedAt))).
		WithHtml(fmt.Sprintf("<li><strong>Denied At:</strong> %s</li>", formatUTCTime(mail.Request.ApprovedAt))).
		WithHtml(fmt.Sprintf("<li><strong>Denied By:</strong> %s</li>", mail.Request.ApprovedBy)).
		WithHtml("</ul>").
		WithParagraph("If you believe you need access to this network, please contact your network administrator or submit a new JIT access request.").
		WithParagraph("Best Regards,").
		WithParagraph("The Netmaker Team").
		Build()

	return content
}
