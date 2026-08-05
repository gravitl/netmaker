package email

import "fmt"

// EmailValidationMail - mail sent to a user to verify ownership of their email/username.
type EmailValidationMail struct {
	BodyBuilder EmailBodyBuilder
	ValidateURL string
}

// GetSubject - gets the subject of the email
func (EmailValidationMail) GetSubject(info Notification) string {
	return "Verify Your Email for Netmaker"
}

// GetBody - gets the body of the email
func (m EmailValidationMail) GetBody(info Notification) string {
	supportEmail := "support@netmaker.io"

	return m.BodyBuilder.
		WithParagraph("Hi,").
		WithParagraph("Please verify your email address to finish setting up your Netmaker account.").
		WithHtml(fmt.Sprintf("<p><a href=\"%s\">Click here to verify your email</a>.</p>", m.ValidateURL)).
		WithParagraph(fmt.Sprintf("If you have any questions or need assistance, please contact our support team at <a href=\"mailto:%s\">%s</a>.", supportEmail, supportEmail)).
		WithParagraph("Best Regards,").
		WithParagraph("The Netmaker Team").
		Build()
}
