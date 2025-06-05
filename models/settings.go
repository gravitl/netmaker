package models

type Theme string

const (
	Dark   Theme = "dark"
	Light  Theme = "light"
	System Theme = "system"
)

type ServerSettings struct {
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
	UserFilters                    []string `json:"user_filters"`
	GroupFilters                   []string `json:"group_filters"`
	IDPSyncInterval                string   `json:"idp_sync_interval"`
	Telemetry                      string   `json:"telemetry"`
	BasicAuth                      bool     `json:"basic_auth"`
	JwtValidityDuration            int      `json:"jwt_validity_duration"`
	RacAutoDisable                 bool     `json:"rac_auto_disable"`
	RacRestrictToSingleNetwork     bool     `json:"rac_restrict_to_single_network"`
	EndpointDetection              bool     `json:"endpoint_detection"`
	AllowedEmailDomains            string   `json:"allowed_email_domains"`
	EmailSenderAddr                string   `json:"email_sender_addr"`
	EmailSenderUser                string   `json:"email_sender_user"`
	EmailSenderPassword            string   `json:"email_sender_password"`
	SmtpHost                       string   `json:"smtp_host"`
	SmtpPort                       int      `json:"smtp_port"`
	MetricInterval                 string   `json:"metric_interval"`
	MetricsPort                    int      `json:"metrics_port"`
	ManageDNS                      bool     `json:"manage_dns"`
	DefaultDomain                  string   `json:"default_domain"`
	Stun                           bool     `json:"stun"`
	StunServers                    string   `json:"stun_servers"`
	Theme                          Theme    `json:"theme"`
	TextSize                       string   `json:"text_size"`
	ReducedMotion                  bool     `json:"reduced_motion"`
	AuditLogsRetentionPeriodInDays int      `json:"audit_logs_retention_period"`
}
