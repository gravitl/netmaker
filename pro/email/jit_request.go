package email

import (
	"fmt"
	"time"

	"github.com/gravitl/netmaker/models"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/gravitl/netmaker/schema"
)

// JITRequestMail - mail for notifying admins of JIT access requests
type JITRequestMail struct {
	BodyBuilder EmailBodyBuilder
	Request     *schema.JITRequest
	Network     models.Network
}

// GetSubject - gets the subject of the email
func (mail JITRequestMail) GetSubject(info Notification) string {
	return fmt.Sprintf("JIT Access Request: %s requests access to %s", mail.Request.UserName, mail.Network.NetID)
}

// GetBody - gets the body of the email
func (mail JITRequestMail) GetBody(info Notification) string {
	dashboardURL := fmt.Sprintf("https://dashboard.%s/networks/%s", servercfg.GetNmBaseDomain(), mail.Network.NetID)
	if servercfg.DeployedByOperator() {
		dashboardURL = fmt.Sprintf("%s/dashboard?tenant_id=%s&network=%s", 
			proLogic.GetAccountsUIHost(), servercfg.GetNetmakerTenantID(), mail.Network.NetID)
	}

	reasonText := mail.Request.Reason
	if reasonText == "" {
		reasonText = "No reason provided"
	}

	content := mail.BodyBuilder.
		WithHeadline("New JIT Access Request").
		WithParagraph(fmt.Sprintf("User <strong>%s</strong> has requested Just-In-Time access to network <strong>%s</strong>.", 
			mail.Request.UserName, mail.Network.NetID)).
		WithParagraph("Request Details:").
		WithHtml("<ul>").
		WithHtml(fmt.Sprintf("<li><strong>User:</strong> %s</li>", mail.Request.UserName)).
		WithHtml(fmt.Sprintf("<li><strong>Network:</strong> %s</li>", mail.Network.NetID)).
		WithHtml(fmt.Sprintf("<li><strong>Requested At:</strong> %s</li>", mail.Request.RequestedAt.Format(time.RFC3339))).
		WithHtml(fmt.Sprintf("<li><strong>Reason:</strong> %s</li>", reasonText)).
		WithHtml("</ul>").
		WithParagraph(fmt.Sprintf("<a href=\"%s\" style=\"display: inline-block; padding: 12px 24px; background-color: #007bff; color: #ffffff; text-decoration: none; border-radius: 4px;\">Review Request</a>", dashboardURL)).
		WithParagraph("You can approve or deny this request from the network settings page.").
		WithParagraph("Best Regards,").
		WithParagraph("The Netmaker Team").
		Build()

	return content
}

