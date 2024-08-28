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

	smtpSender := &SmtpSender{
		SmtpHost:    servercfg.GetSmtpHost(),
		SmtpPort:    servercfg.GetSmtpPort(),
		SenderEmail: servercfg.GetSenderEmail(),
		SendUser:    servercfg.GetSenderUser(),
		SenderPass:  servercfg.GetEmaiSenderPassword(),
	}
	if smtpSender.SendUser == "" {
		smtpSender.SendUser = smtpSender.SenderEmail
	}
	client = smtpSender

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
