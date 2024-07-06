package logic

import (
	"crypto/tls"
	"fmt"
	"net/url"

	gomail "gopkg.in/mail.v2"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

var (
	smtpHost       = servercfg.GetSmtpHost()
	smtpPort       = servercfg.GetSmtpPort()
	senderEmail    = servercfg.GetSenderEmail()
	senderPassword = servercfg.GetSenderEmailPassWord()
)

func SendInviteEmail(invite models.UserInvite) error {
	m := gomail.NewMessage()

	// Set E-Mail sender
	m.SetHeader("From", senderEmail)

	// Set E-Mail receivers
	m.SetHeader("To", invite.Email)

	// Set E-Mail subject
	m.SetHeader("Subject", "Netmaker Invite")

	// Set E-Mail body. You can set plain text or html with text/html
	u, err := url.Parse(fmt.Sprintf("https://api.%s/api/v1/users/invite?email=%s&code=%s",
		servercfg.GetServer(), invite.Email, invite.InviteCode))
	if err != nil {
		return err
	}
	m.SetBody("text/html", "Click Here to Signup! <a>"+u.String())

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
