package email

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

// JITApprovedMail - mail for notifying users when their JIT request is approved
type JITApprovedMail struct {
	BodyBuilder EmailBodyBuilder
	Grant       *schema.JITGrant
	Request     *schema.JITRequest
	Network     models.Network
}

// SendJITApprovalEmail - sends email notification to user when JIT request is approved
func SendJITApprovalEmail(grant *schema.JITGrant, request *schema.JITRequest, network models.Network) error {
	mail := JITApprovedMail{
		BodyBuilder: &EmailBodyBuilderWithH1HeadlineAndImage{},
		Grant:       grant,
		Request:     request,
		Network:     network,
	}
	// Skip sending email if username is not a valid email address
	if !IsValid(request.UserName) {
		slog.Warn("skipping JIT request approval email with non-email username", "user", request.UserName)
		return nil
	}
	notification := Notification{
		RecipientMail: request.UserName, // Assuming username is email
		RecipientName: request.UserName,
	}

	return GetClient().SendEmail(context.Background(), notification, mail)
}

// GetSubject - gets the subject of the email
func (mail JITApprovedMail) GetSubject(info Notification) string {
	return fmt.Sprintf("JIT Access Approved: %s", mail.Network.NetID)
}

// GetBody - gets the body of the email
func (mail JITApprovedMail) GetBody(info Notification) string {
	content := mail.BodyBuilder.
		WithHeadline("JIT Access Approved").
		WithParagraph(fmt.Sprintf("Your request for Just-In-Time access to network <strong>%s</strong> has been approved.", mail.Network.NetID)).
		WithParagraph("Access Details:").
		WithHtml("<ul>").
		WithHtml(fmt.Sprintf("<li><strong>Network:</strong> %s</li>", mail.Network.NetID)).
		WithHtml(fmt.Sprintf("<li><strong>Granted At:</strong> %s</li>", formatUTCTime(mail.Grant.GrantedAt))).
		WithHtml(fmt.Sprintf("<li><strong>Expires At:</strong> %s</li>", formatUTCTime(mail.Grant.ExpiresAt))).
		WithHtml(fmt.Sprintf("<li><strong>Approved By:</strong> %s</li>", mail.Request.ApprovedBy)).
		WithHtml("</ul>").
		WithParagraph("You can now connect to this network using the Netmaker Desktop App. Your access will automatically expire when the granted duration ends.").
		WithParagraph("Best Regards,").
		WithParagraph("The Netmaker Team").
		Build()

	return content
}
