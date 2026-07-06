package schema

import (
	"context"

	"github.com/gravitl/netmaker/db"
	"gorm.io/datatypes"
)

type TenantSettings struct {
	NetclientAutoUpdate            bool     `json:"netclientautoupdate"`
	Verbosity                      int32    `json:"verbosity"`
	AuthProvider                   string   `json:"authprovider"`
	OIDCIssuer                     string   `json:"oidcissuer"`
	ClientID                       string   `json:"client_id"`
	ClientSecret                   string   `json:"client_secret"`
	SyncEnabled                    bool     `json:"sync_enabled"`
	GoogleAdminEmail               string   `json:"google_admin_email"`
	GoogleSACredsJson              string   `json:"google_sa_creds_json"`
	AzureTenant                    string   `json:"azure_tenant"`
	OktaOrgURL                     string   `json:"okta_org_url"`
	OktaAPIToken                   string   `json:"okta_api_token"`
	UserFilters                    []string `json:"user_filters"`
	GroupFilters                   []string `json:"group_filters"`
	IDPSyncInterval                string   `json:"idp_sync_interval"`
	Telemetry                      string   `json:"telemetry"`
	BasicAuth                      bool     `json:"basic_auth"`
	JwtValidityDuration            int      `json:"jwt_validity_duration"`
	JwtValidityDurationClients     int      `json:"jwt_validity_duration_clients"`
	MFAEnforced                    bool     `json:"mfa_enforced"`
	RacRestrictToSingleNetwork     bool     `json:"rac_restrict_to_single_network"`
	EndpointDetection              bool     `json:"endpoint_detection"`
	AllowedEmailDomains            string   `json:"allowed_email_domains"`
	EmailSenderAddr                string   `json:"email_sender_addr"`
	EmailSenderUser                string   `json:"email_sender_user"`
	EmailSenderPassword            string   `json:"email_sender_password"`
	SmtpHost                       string   `json:"smtp_host"`
	SmtpPort                       int      `json:"smtp_port"`
	SmtpSkipTlsVerify              bool     `json:"smtp_skip_tls_verify"`
	MetricInterval                 string   `json:"metric_interval"`
	MetricsPort                    int      `json:"metrics_port"`
	IPDetectionInterval            int      `json:"ip_detection_interval"`
	ManageDNS                      bool     `json:"manage_dns"`
	DefaultDomain                  string   `json:"default_domain"`
	Stun                           bool     `json:"stun"`
	StunServers                    string   `json:"stun_servers"`
	AuditLogsRetentionPeriodInDays int      `json:"audit_logs_retention_period"`
	PeerConnectionCheckInterval    string   `json:"peer_connection_check_interval"`
	PostureCheckInterval           string   `json:"posture_check_interval"`
	CleanUpInterval                int      `json:"clean_up_interval_in_mins"`
	EnableFlowLogs                 bool     `json:"enable_flow_logs"`
}

type TenantSettingsRecord struct {
	Key   string `gorm:"primaryKey"`
	Value datatypes.JSONType[TenantSettings]
}

func (*TenantSettingsRecord) TableName() string { return "server_settings" }

func (r *TenantSettingsRecord) Get(ctx context.Context) error {
	return db.FromContext(ctx).First(r).Error
}

func (r *TenantSettingsRecord) Upsert(ctx context.Context) error {
	return db.FromContext(ctx).Save(r).Error
}

func (r *TenantSettingsRecord) Delete(ctx context.Context) error {
	return db.FromContext(ctx).Delete(r).Error
}

func (*TenantSettingsRecord) List(ctx context.Context) ([]TenantSettingsRecord, error) {
	var records []TenantSettingsRecord
	err := db.FromContext(ctx).Find(&records).Error
	return records, err
}

func (*TenantSettingsRecord) Count(ctx context.Context) (int, error) {
	var count int64
	err := db.FromContext(ctx).Model(&TenantSettingsRecord{}).Count(&count).Error
	return int(count), err
}
