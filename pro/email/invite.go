package email

import (
	"fmt"
	"github.com/gravitl/netmaker/servercfg"
)

// UserInvitedMail - mail for users that are invited to a tenant
type UserInvitedMail struct {
	BodyBuilder EmailBodyBuilder
	InviteURL   string
}

// GetSubject - gets the subject of the email
func (UserInvitedMail) GetSubject(info Notification) string {
	return "Access Your Secure Network with Netmaker"
}

// GetBody - gets the body of the email
func (invite UserInvitedMail) GetBody(info Notification) string {
	downloadLink := "https://www.netmaker.io/download"
	racDocsLink := "https://docs.v2.netmaker.io/guide/netmaker-professional/netmaker-remote-access-client-rac"
	supportEmail := "support@netmaker.io"
	return invite.BodyBuilder.
		WithParagraph("Hi,").
		WithParagraph("You've been invited to access a secure network via Netmaker's Remote Access Client (RAC). Follow these simple steps to get connected:").
		WithHtml("<ol>").
		WithHtml("<li>Accept Invite - Click the button to accept your invitation.</li>").
		WithHtml("<br>").
		WithHtml(fmt.Sprintf("<a style=\"background:#5E5DF0; border-radius:999px; box-shadow:#5E5DF0 0 10px 20px -10px; box-sizing:border-box; color:#FFFFFF !important; cursor:pointer; font-family:Helvetica; font-size:16px; font-weight:700; line-height:24px; opacity:1; outline:0 solid transparent; padding:8px 18px; user-select:none; -webkit-user-select:none; touch-action:manipulation; width:fit-content; word-break:break-word; border:0; margin:20px 20px 20px 0px; text-decoration:none;\" href=\"%s\">Accept Invite</a>", invite.InviteURL)).
		WithHtml("<br><br>").
		WithHtml(fmt.Sprintf("<li>Download the Remote Access Client (RAC). Visit our download page to get the RAC for your device: <a href=\"%s\">%s</a>.</li>", downloadLink, downloadLink)).
		WithHtml("<br>").
		WithHtml("<li>Choose the appropriate version for your operating system.</li>").
		WithHtml("</ol>").
		WithParagraph(fmt.Sprintf("Important: Your Tenant ID is %s. You may need this for troubleshooting or support requests.", servercfg.GetNetmakerTenantID())).
		WithParagraph(fmt.Sprintf("For detailed setup instructions and troubleshooting, please visit our RAC user guide: <a href=\"%s\">%s</a>.", racDocsLink, racDocsLink)).
		WithParagraph(fmt.Sprintf("If you have any questions or need assistance, don't hesitate to contact our support team at <a href=\"mailto:%s\">%s</a>.", supportEmail, supportEmail)).
		WithParagraph("Welcome aboard, and enjoy your secure connection!").
		WithParagraph("Best regards,").
		WithParagraph("The Netmaker Team").
		Build()
}
