package email

import (
	"context"
	"regexp"
	"sync"

	"github.com/gravitl/netmaker/logic"
)

type EmailSenderType string

var (
	client    EmailSender
	clientMtx sync.RWMutex
)

const (
	Smtp   EmailSenderType = "smtp"
	Resend EmailSenderType = "resend"
)

func Init(ctx context.Context) {

	smtpSender := &SmtpSender{
		SmtpHost:    logic.GetSmtpHost(ctx),
		SmtpPort:    logic.GetSmtpPort(ctx),
		SenderEmail: logic.GetSenderEmail(ctx),
		SendUser:    logic.GetSenderUser(ctx),
		SenderPass:  logic.GetEmaiSenderPassword(ctx),
		SkipVerify:  logic.SmtpSkipTlsVerify(ctx),
	}
	if smtpSender.SendUser == "" {
		smtpSender.SendUser = smtpSender.SenderEmail
	}
	clientMtx.Lock()
	client = smtpSender
	clientMtx.Unlock()
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
	clientMtx.RLock()
	defer clientMtx.RUnlock()
	if client == nil {
		return &noOpSender{}
	}
	return client
}

func IsValid(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)
	return emailRegex.MatchString(email)
}
