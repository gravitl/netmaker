package email

import (
	"context"
	"fmt"

	"github.com/gravitl/netmaker/servercfg"
	"github.com/resendlabs/resend-go"
)

// ResendEmailSender - implementation of EmailSender using Resend (https://resend.com)
type ResendEmailSender struct {
	client ResendClient
	from   string
}

// ResendClient - dependency interface for resend client
type ResendClient interface {
	Send(*resend.SendEmailRequest) (resend.SendEmailResponse, error)
}

// NewResendEmailSender - constructs a ResendEmailSender
func NewResendEmailSender(client ResendClient, from string) ResendEmailSender {
	return ResendEmailSender{client: client, from: from}
}

// NewResendEmailSender - constructs a ResendEmailSender from config
// TODO let main.go handle this and use dependency injection instead of calling this function
func NewResendEmailSenderFromConfig() ResendEmailSender {
	key, from := servercfg.GetEmaiSenderAuth(), servercfg.GetSenderEmail()
	resender := resend.NewClient(key)
	return NewResendEmailSender(resender.Emails, from)
}

// SendEmail - sends an email using resend-go (https://github.com/resendlabs/resend-go)
func (es ResendEmailSender) SendEmail(ctx context.Context, notification Notification, email Mail) error {
	var (
		from    = es.from
		to      = notification.RecipientMail
		subject = email.GetSubject(notification)
		body    = email.GetBody(notification)
	)
	params := resend.SendEmailRequest{
		From:    from,
		To:      []string{to},
		Subject: subject,
		Html:    body,
	}
	_, err := es.client.Send(&params)
	if err != nil {
		return fmt.Errorf("failed sending mail via resend: %w", err)
	}

	return nil
}
