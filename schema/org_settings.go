package schema

import (
	"context"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

const orgSettingsTable = "org_settings_v1"

type OrganizationSettings struct {
	ID       string `gorm:"primaryKey"`
	Settings datatypes.JSONType[OrganizationSettingsData]
}

type OrganizationSettingsData struct {
	AuthProvider                   string `json:"auth_provider"`
	ClientID                       string `json:"client_id"`
	ClientSecret                   string `json:"client_secret"`
	OIDCIssuer                     string `json:"oidc_issuer"`
	AzureTenant                    string `json:"azure_tenant"`
	EmailSenderAddr                string `json:"email_sender_addr"`
	EmailSenderUser                string `json:"email_sender_user"`
	EmailSenderPassword            string `json:"email_sender_password"`
	SmtpHost                       string `json:"smtp_host"`
	SmtpPort                       int    `json:"smtp_port"`
	SmtpSkipTlsVerify              bool   `json:"smtp_skip_tls_verify"`
	AuditLogsRetentionPeriodInDays int    `json:"audit_logs_retention_period"`
}

func (o *OrganizationSettings) TableName() string {
	return orgSettingsTable
}

func (o *OrganizationSettings) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(&o).Error
}

func (o *OrganizationSettings) Get(ctx context.Context) error {
	return db.FromContext(ctx).Model(&OrganizationSettings{}).
		Where("id = ?", o.ID).
		First(&o).
		Error
}

func (o *OrganizationSettings) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Delete(&o).Error
}
