package models

import "time"

type ServerSettings struct {
	NetclientAutoUpdate        bool          `json:"netclientautoupdate"`
	Verbosity                  int32         `json:"verbosity"`
	AuthProvider               string        `json:"authprovider"`
	OIDCIssuer                 string        `json:"oidcissuer"`
	ClientID                   string        `json:"client_id"`
	ClientSecret               string        `json:"client_secret"`
	AzureTenant                string        `json:"azure_tenant"`
	Telemetry                  string        `json:"telemetry"`
	BasicAuth                  bool          `json:"basic_auth"`
	JwtValidityDuration        time.Duration `json:"jwt_validity_duration" swaggertype:"primitive,integer" format:"int64"`
	RacAutoDisable             bool          `json:"rac_auto_disable"`
	RacRestrictToSingleNetwork bool          `json:"rac_restrict_to_single_network"`
	EndpointDetection          bool          `json:"endpoint_detection"`
	AllowedEmailDomains        string        `json:"allowed_email_domains"`
	EmailSenderAddr            string        `json:"email_sender_addr"`
	EmailSenderUser            string        `json:"email_sender_user"`
	EmailSenderPassword        string        `json:"email_sender_password"`
	SmtpHost                   string        `json:"smtp_host"`
	SmtpPort                   int           `json:"smtp_port"`
	MetricInterval             string        `json:"metric_interval"`
	MetricsPort                int           `json:"metrics_port"`
	ManageDNS                  bool          `json:"manage_dns"`
	DefaultDomain              string        `json:"default_domain"`
	Stun                       bool          `json:"stun"`
	StunServers                string        `json:"stun_servers"`
}
