package email

import (
	"fmt"

	"github.com/gravitl/netmaker/logic"
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
		dashboardURL = fmt.Sprintf("%s/dashboard?tenant_id=%s", proLogic.GetAccountsUIHost(), logic.GetNetmakerTenantID())
	}

	content := invite.BodyBuilder.
		WithParagraph("Hi,").
		WithParagraph("You've been invited to access a secure network via Netmaker Desktop App. Follow these simple steps to get connected:").
		WithHtml("<ol>").
		WithHtml(fmt.Sprintf("<li>Click <a href=\"%s\">here</a> to accept your invitation and setup your account.</li>", invite.InviteURL)).
		WithHtml("<br>").
		WithHtml(fmt.Sprintf("<li><a href=\"%s\">Download the Netmaker Desktop App</a>.</li>", downloadLink))

	if invite.PlatformRoleID == models.AdminRole.String() || invite.PlatformRoleID == models.PlatformUser.String() {
		content = content.
			WithHtml("<br>").
			WithHtml(fmt.Sprintf("<li>Access the <a href=\"%s\">Netmaker Dashboard</a> - use it to manage your network settings and view network status.</li>", dashboardURL))
	}

	connectionID := logic.GetNetmakerTenantID()
	if !servercfg.DeployedByOperator() {
		connectionID = fmt.Sprintf("api.%s", servercfg.GetNmBaseDomain())
	}

	return content.
		WithHtml("</ol>").
		WithParagraph("Important Information:").
		WithHtml("<ul>").
		WithHtml(fmt.Sprintf("<li>When connecting through RAC, please enter your server connection ID: %s.</li>", connectionID)).
		WithHtml("</ul>").
		WithParagraph(fmt.Sprintf("If you have any questions or need assistance, please contact our support team at <a href=\"mailto:%s\">%s</a>.", supportEmail, supportEmail)).
		WithParagraph("Best Regards,").
		WithParagraph("The Netmaker Team").
		Build()
}
