package email

import (
	"fmt"
)

// UserInvitedMail - mail for users that are invited to a tenant
type UserInvitedMail struct {
	BodyBuilder EmailBodyBuilder
	InviteURL   string
}

// GetSubject - gets the subject of the email
func (UserInvitedMail) GetSubject(info Notification) string {
	return "Netmaker SaaS: Invitation Pending Acceptance"
}

// GetBody - gets the body of the email
func (invite UserInvitedMail) GetBody(info Notification) string {

	return invite.BodyBuilder.
		WithHeadline("Join Netmaker from this invite!").
		WithParagraph("Hello from Netmaker,").
		WithParagraph("You have been invited to join Netmaker.").
		WithParagraph(fmt.Sprintf("Join Using This Invite Link <a href=\"%s\">Netmaker</a>", invite.InviteURL)).
		Build()
}
