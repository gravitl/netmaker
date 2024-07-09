package email

import (
	"crypto/tls"

	gomail "gopkg.in/mail.v2"

	"github.com/gravitl/netmaker/servercfg"
)

var (
	smtpHost       = servercfg.GetSmtpHost()
	smtpPort       = servercfg.GetSmtpPort()
	senderEmail    = servercfg.GetSenderEmail()
	senderPassword = servercfg.GetSenderEmailPassWord()
)

type Email interface {
	GetBody(info Notification) string
	GetSubject(info Notification) string
}

// Notification - struct for notification details
type Notification struct {
	RecipientMail string
	RecipientName string
	ProductName   string
}

func (n Notification) NewEmailSender(e Email) *gomail.Message {
	m := gomail.NewMessage()

	// Set E-Mail sender
	m.SetHeader("From", senderEmail)

	// Set E-Mail receivers
	m.SetHeader("To", n.RecipientMail)
	// Set E-Mail subject
	m.SetHeader("Subject", e.GetSubject(n))
	// Set E-Mail body. You can set plain text or html with text/html
	m.SetBody("text/html", e.GetBody(n))

	return m
}

func Send(m *gomail.Message) error {

	// Settings for SMTP server
	d := gomail.NewDialer(smtpHost, smtpPort, senderEmail, senderPassword)

	// This is only needed when SSL/TLS certificate is not valid on server.
	// In production this should be set to false.
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	// Now send E-Mail
	if err := d.DialAndSend(m); err != nil {
		return err
	}

	return nil
}
