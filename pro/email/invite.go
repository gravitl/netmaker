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
	return "You're invited to join Netmaker"
}

// GetBody - gets the body of the email
func (invite UserInvitedMail) GetBody(info Notification) string {
	if servercfg.DeployedByOperator() {
		return invite.BodyBuilder.
			WithParagraph("Hi there,").
			WithParagraph("<br>").
			WithParagraph("Great news! Your colleague has invited you to join their Netmaker SaaS Tenant.").
			WithParagraph("Click the button to accept your invitation:").
			WithParagraph("<br>").
			WithParagraph(fmt.Sprintf("<a class=\"x-button\" href=\"%s\">Accept Invitation</a>", invite.InviteURL)).
			WithParagraph("<br>").
			WithParagraph("Why you'll love Netmaker:").
			WithParagraph("<ul>").
			WithParagraph("<li>Blazing-fast connections with our WireGuard®-powered mesh VPN</li>").
			WithParagraph("<li>Seamless multi-cloud and hybrid-cloud networking</li>").
			WithParagraph("<li>Automated Kubernetes networking across any infrastructure</li>").
			WithParagraph("<li>Enterprise-grade security with simple management</li>").
			WithParagraph("</ul>").
			WithParagraph("Got questions? Our team is here to help you every step of the way.").
			WithParagraph("<br>").
			WithParagraph("Welcome aboard,").
			WithParagraph("<h2>The Netmaker Team</h2>").
			WithParagraph("P.S. Curious to learn more before accepting? Check out our quick start tutorial at <a href=\"https://netmaker.io/tutorials\">netmaker.io/tutorials</a>").
			Build()
	}

	return invite.BodyBuilder.
		WithParagraph("Hi there,").
		WithParagraph("<br>").
		WithParagraph("Great news! Your colleague has invited you to join their Netmaker network.").
		WithParagraph("Click the button to accept your invitation:").
		WithParagraph("<br>").
		WithParagraph(fmt.Sprintf("<a class=\"x-button\" href=\"%s\">Accept Invitation</a>", invite.InviteURL)).
		WithParagraph("<br>").
		WithParagraph("Why you'll love Netmaker:").
		WithParagraph("<ul>").
		WithParagraph("<li>Blazing-fast connections with our WireGuard®-powered mesh VPN</li>").
		WithParagraph("<li>Seamless multi-cloud and hybrid-cloud networking</li>").
		WithParagraph("<li>Automated Kubernetes networking across any infrastructure</li>").
		WithParagraph("<li>Enterprise-grade security with simple management</li>").
		WithParagraph("</ul>").
		WithParagraph("Got questions? Our team is here to help you every step of the way.").
		WithParagraph("<br>").
		WithParagraph("Welcome aboard,").
		WithParagraph("<h2>The Netmaker Team</h2>").
		WithParagraph("P.S. Curious to learn more before accepting? Check out our quick start tutorial at <a href=\"https://netmaker.io/tutorials\">netmaker.io/tutorials</a>").
		Build()
}
