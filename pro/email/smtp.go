package email

import (
	"context"
	"crypto/tls"

	gomail "gopkg.in/mail.v2"
)

type SmtpSender struct {
	SmtpHost    string
	SmtpPort    int
	SenderEmail string
	SendUser    string
	SenderPass  string
}

func (s *SmtpSender) SendEmail(ctx context.Context, n Notification, e Mail) error {
	m := gomail.NewMessage()

	// Set E-Mail sender
	m.SetHeader("From", s.SenderEmail)

	// Set E-Mail receivers
	m.SetHeader("To", n.RecipientMail)
	// Set E-Mail subject
	m.SetHeader("Subject", e.GetSubject(n))
	// Set E-Mail body. You can set plain text or html with text/html
	m.SetBody("text/html", e.GetBody(n))
	// Settings for SMTP server
	d := gomail.NewDialer(s.SmtpHost, s.SmtpPort, s.SendUser, s.SenderPass)

	// This is only needed when SSL/TLS certificate is not valid on server.
	// In production this should be set to false.
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	// Now send E-Mail
	if err := d.DialAndSend(m); err != nil {
		return err
	}

	return nil
}
