package email

import (
	"fmt"
	"github.com/gravitl/netmaker/models"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"github.com/gravitl/netmaker/servercfg"
)

// UserInvitedMail - mail for users that are invited to a tenant
type UserInvitedMail struct {
	BodyBuilder    EmailBodyBuilder
	InviteURL      string
	PlatformRoleID string
}

// GetSubject - gets the subject of the email
func (UserInvitedMail) GetSubject(info Notification) string {
	return "Connect to Your Secure Network Using Netmaker"
}

// GetBody - gets the body of the email
func (invite UserInvitedMail) GetBody(info Notification) string {
	downloadLink := "https://www.netmaker.io/download"
	supportEmail := "support@netmaker.io"

	dashboardURL := fmt.Sprintf("https://dashboard.%s", servercfg.GetNmBaseDomain())
	if servercfg.DeployedByOperator() {
		dashboardURL = fmt.Sprintf("%s/dashboard?tenant_id=%s", proLogic.GetAccountsUIHost(), servercfg.GetNetmakerTenantID())
	}

	content := invite.BodyBuilder.
		WithParagraph("Hi,").
		WithParagraph("You've been invited to access a secure network via Netmaker's Remote Access Client (RAC). Follow these simple steps to get connected:").
		WithHtml("<ol>").
		WithHtml(fmt.Sprintf("<li>Click to <a href=\"%s\">Accept Your Invitation</a> and setup your account.</li>", invite.InviteURL)).
		WithHtml("<br>").
		WithHtml(fmt.Sprintf("<li><a href=\"%s\">Download the Remote Access Client (RAC)</a>.</li>", downloadLink))

	if invite.PlatformRoleID == models.AdminRole.String() || invite.PlatformRoleID == models.PlatformUser.String() {
		content = content.
			WithHtml("<br>").
			WithHtml(fmt.Sprintf("<li>Access the <a href=\"%s\">Netmaker Dashboard</a> - use it to manage your network settings and view network status.</li>", dashboardURL))
	}

	content = content.
		WithHtml("</ol>").
		WithParagraph("Important Information:").
		WithHtml("<ul>")

	if servercfg.DeployedByOperator() {
		content = content.
			WithHtml(fmt.Sprintf("<li>Your Tenant ID: %s</li>", servercfg.GetNetmakerTenantID()))
	} else {
		content = content.
			WithHtml(fmt.Sprintf("<li>Your Netmaker Domain: %s</li>", fmt.Sprintf("api.%s", servercfg.GetNmBaseDomain())))
	}

	return content.
		WithHtml("</ul>").
		WithParagraph(fmt.Sprintf("If you have any questions or need assistance, please contact our support team at <a href=\"mailto:%s\">%s</a>.", supportEmail, supportEmail)).
		WithParagraph("Best regards,").
		WithParagraph("The Netmaker Team").
		Build()
}
