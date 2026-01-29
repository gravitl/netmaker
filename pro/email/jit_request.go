package email

import (
	"context"
	"fmt"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
)

// JITRequestMail - mail for notifying admins of JIT access requests
type JITRequestMail struct {
	BodyBuilder EmailBodyBuilder
	Request     *schema.JITRequest
	Network     models.Network
}

// SendJITRequestEmails - sends email notifications to network admins about JIT requests
func SendJITRequestEmails(request *schema.JITRequest, network models.Network) error {
	admins, err := proLogic.GetNetworkAdmins(request.NetworkID)
	if err != nil {
		return err
	}

	mail := JITRequestMail{
		BodyBuilder: &EmailBodyBuilderWithH1HeadlineAndImage{},
		Request:     request,
		Network:     network,
	}

	for _, admin := range admins {
		if admin.UserName == "" {
			continue
		}

		// Skip sending email if username is not a valid email address
		if !IsValid(admin.UserName) {
			logger.Log(2, "skipping JIT request email for admin with non-email username", "admin", admin.UserName)
			continue
		}

		notification := Notification{
			RecipientMail: admin.UserName, // Assuming username is email
			RecipientName: admin.UserName,
		}

		if err := GetClient().SendEmail(context.Background(), notification, mail); err != nil {
			logger.Log(0, "failed to send JIT request email", "admin", admin.UserName, "error", err.Error())
			continue
		}
	}

	return nil
}

// GetSubject - gets the subject of the email
func (mail JITRequestMail) GetSubject(info Notification) string {
	return fmt.Sprintf("JIT Access Request: %s requests access to %s", mail.Request.UserName, mail.Network.NetID)
}

// GetBody - gets the body of the email
func (mail JITRequestMail) GetBody(info Notification) string {
	dashboardURL := fmt.Sprintf("https://dashboard.%s/networks/%s/jit-requests?jit_req_id=%s", servercfg.GetNmBaseDomain(),
		mail.Network.NetID, mail.Request.ID)
	if servercfg.DeployedByOperator() {
		dashboardURL = fmt.Sprintf("%s/dashboard?tenant_id=%s&network=%s&jit_req_id=%s",
			proLogic.GetAccountsUIHost(), servercfg.GetNetmakerTenantID(), mail.Network.NetID, mail.Request.ID)
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
		WithHtml(fmt.Sprintf("<li><strong>Requested At:</strong> %s</li>", formatUTCTime(mail.Request.RequestedAt))).
		WithHtml(fmt.Sprintf("<li><strong>Reason:</strong> %s</li>", reasonText)).
		WithHtml("</ul>").
		WithParagraph(fmt.Sprintf("<a href=\"%s\" style=\"display: inline-block; padding: 12px 24px; background-color: #007bff; color: #ffffff; text-decoration: none; border-radius: 4px;\">Review Request</a>", dashboardURL)).
		WithParagraph("You can approve or deny this request from the network JIT page.").
		WithParagraph("Best Regards,").
		WithParagraph("The Netmaker Team").
		Build()

	return content
}
