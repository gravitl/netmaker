package email

import (
	"context"

	"github.com/gravitl/netmaker/servercfg"
)

type EmailSenderType string

var client EmailSender

const (
	Smtp   EmailSenderType = "smtp"
	Resend EmailSenderType = "resend"
)

func init() {
	switch EmailSenderType(servercfg.EmailSenderType()) {
	case Smtp:
		smtpSender := &SmtpSender{
			SmtpHost:    servercfg.GetSmtpHost(),
			SmtpPort:    servercfg.GetSmtpPort(),
			SenderEmail: servercfg.GetSenderEmail(),
			SendUser:    servercfg.GetSenderUser(),
			SenderPass:  servercfg.GetEmaiSenderAuth(),
		}
		if smtpSender.SendUser == "" {
			smtpSender.SendUser = smtpSender.SenderEmail
		}
		client = smtpSender

	case Resend:
		client = NewResendEmailSenderFromConfig()
	}
	client = GetClient()
}

// EmailSender - an interface for sending emails based on notifications and mail templates
type EmailSender interface {
	// SendEmail - sends an email based on a context, notification and mail template
	SendEmail(ctx context.Context, notification Notification, email Mail) error
}

type Mail interface {
	GetBody(info Notification) string
	GetSubject(info Notification) string
}

// Notification - struct for notification details
type Notification struct {
	RecipientMail string
	RecipientName string
	ProductName   string
}

func GetClient() (e EmailSender) {
	return client
}
