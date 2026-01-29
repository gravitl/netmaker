package models

type Theme string

const (
	Dark   Theme = "dark"
	Light  Theme = "light"
	System Theme = "system"
)

type ServerSettings struct {
	NetclientAutoUpdate bool     `json:"netclientautoupdate"`
	Verbosity           int32    `json:"verbosity"`
	AuthProvider        string   `json:"authprovider"`
	OIDCIssuer          string   `json:"oidcissuer"`
	ClientID            string   `json:"client_id"`
	ClientSecret        string   `json:"client_secret"`
	SyncEnabled         bool     `json:"sync_enabled"`
	GoogleAdminEmail    string   `json:"google_admin_email"`
	GoogleSACredsJson   string   `json:"google_sa_creds_json"`
	AzureTenant         string   `json:"azure_tenant"`
	OktaOrgURL          string   `json:"okta_org_url"`
	OktaAPIToken        string   `json:"okta_api_token"`
	UserFilters         []string `json:"user_filters"`
	GroupFilters        []string `json:"group_filters"`
	IDPSyncInterval     string   `json:"idp_sync_interval"`
	Telemetry           string   `json:"telemetry"`
	BasicAuth           bool     `json:"basic_auth"`
	// JwtValidityDuration is the validity duration of auth tokens for users
	// on the dashboard (NMUI).
	JwtValidityDuration int `json:"jwt_validity_duration"`
	// JwtValidityDurationClients is the validity duration of auth tokens for
	// users on the clients (NetDesk).
	JwtValidityDurationClients int    `json:"jwt_validity_duration_clients"`
	MFAEnforced                bool   `json:"mfa_enforced"`
	RacRestrictToSingleNetwork bool   `json:"rac_restrict_to_single_network"`
	EndpointDetection          bool   `json:"endpoint_detection"`
	AllowedEmailDomains        string `json:"allowed_email_domains"`
	EmailSenderAddr            string `json:"email_sender_addr"`
	EmailSenderUser            string `json:"email_sender_user"`
	EmailSenderPassword        string `json:"email_sender_password"`
	SmtpHost                   string `json:"smtp_host"`
	SmtpPort                   int    `json:"smtp_port"`
	MetricInterval             string `json:"metric_interval"`
	MetricsPort                int    `json:"metrics_port"`
	// IPDetectionInterval is the interval (in seconds) at which devices check for changes in public ip.
	IPDetectionInterval            int    `json:"ip_detection_interval"`
	ManageDNS                      bool   `json:"manage_dns"`
	DefaultDomain                  string `json:"default_domain"`
	Stun                           bool   `json:"stun"`
	StunServers                    string `json:"stun_servers"`
	AuditLogsRetentionPeriodInDays int    `json:"audit_logs_retention_period"`
	OldAClsSupport                 bool   `json:"old_acl_support"`
	PeerConnectionCheckInterval    string `json:"peer_connection_check_interval"`
	PostureCheckInterval           string `json:"posture_check_interval"` // in minutes
	CleanUpInterval                int    `json:"clean_up_interval_in_mins"`
	EnableFlowLogs                 bool   `json:"enable_flow_logs"`
}

type UserSettings struct {
	Theme         Theme  `json:"theme"`
	TextSize      string `json:"text_size"`
	ReducedMotion bool   `json:"reduced_motion"`
}
