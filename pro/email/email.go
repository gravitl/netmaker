package email

import (
	"context"
	"regexp"

	"github.com/gravitl/netmaker/logic"
)

type EmailSenderType string

var client EmailSender

const (
	Smtp   EmailSenderType = "smtp"
	Resend EmailSenderType = "resend"
)

func Init() {

	smtpSender := &SmtpSender{
		SmtpHost:    logic.GetSmtpHost(),
		SmtpPort:    logic.GetSmtpPort(),
		SenderEmail: logic.GetSenderEmail(),
		SendUser:    logic.GetSenderUser(),
		SenderPass:  logic.GetEmaiSenderPassword(),
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

func IsValid(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)
	return emailRegex.MatchString(email)
}
