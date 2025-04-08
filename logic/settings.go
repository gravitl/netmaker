package logic

import (
	"encoding/json"

	"github.com/gravitl/netmaker/config"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

var ServerInfo = GetServerInfo()
var serverSettingsDBKey = "server_cfg"

func GetServerSettings() (s models.ServerSettings) {
	data, err := database.FetchRecord(database.SERVER_SETTINGS, serverSettingsDBKey)
	if err != nil {
		return
	}
	json.Unmarshal([]byte(data), &s)
	return
}

func UpsertServerSettings(s models.ServerSettings) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	err = database.Insert(database.SERVER_SETTINGS, string(data), serverSettingsDBKey)
	if err != nil {
		return err
	}
	setServerInfo()
	return nil
}

func ValidateNewSettings(req models.ServerSettings) bool {
	return true
}

func ConvertServerCfgToSettings(c config.ServerConfig) (s models.ServerSettings) {
	s.NetclientAutoUpdate = c.NetclientAutoUpdate
	s.AllowedEmailDomains = c.AllowedEmailDomains
	s.AuthProvider = c.AuthProvider
	s.AzureTenant = c.AzureTenant
	s.BasicAuth = c.BasicAuth
	s.ClientID = c.ClientID
	s.ClientSecret = c.ClientSecret
	s.OIDCIssuer = c.OIDCIssuer
	s.EmailSenderAddr = c.EmailSenderAddr
	s.EmailSenderPassword = c.EmailSenderPassword
	s.EmailSenderUser = c.EmailSenderUser
	s.EndpointDetection = c.EndpointDetection
	s.SmtpHost = c.SmtpHost
	s.SmtpPort = c.SmtpPort
	s.Stun = c.Stun
	s.StunServers = c.StunServers
	s.FrontendURL = c.FrontendURL
	s.JwtValidityDuration = c.JwtValidityDuration
	s.AllowedEmailDomains = c.AllowedEmailDomains
	s.LicenseValue = c.LicenseValue
	s.NetmakerTenantID = c.NetmakerTenantID
	s.ManageDNS = c.ManageDNS
	s.MetricInterval = c.MetricInterval
	s.MetricsPort = c.MetricsPort
	s.NetclientAutoUpdate = c.NetclientAutoUpdate
	s.Telemetry = c.Telemetry
	s.RacAutoDisable = c.RacAutoDisable
	s.RacRestrictToSingleNetwork = c.RacRestrictToSingleNetwork
	return
}

func MergeServerSettingsToServerConfig(s *config.ServerConfig) {
	c := GetServerSettings()

	s.NetclientAutoUpdate = c.NetclientAutoUpdate
	s.AllowedEmailDomains = c.AllowedEmailDomains
	s.AuthProvider = c.AuthProvider
	s.AzureTenant = c.AzureTenant
	s.BasicAuth = c.BasicAuth
	s.ClientID = c.ClientID
	s.ClientSecret = c.ClientSecret
	s.OIDCIssuer = c.OIDCIssuer
	s.EmailSenderAddr = c.EmailSenderAddr
	s.EmailSenderPassword = c.EmailSenderPassword
	s.EmailSenderUser = c.EmailSenderUser
	s.EndpointDetection = c.EndpointDetection
	s.SmtpHost = c.SmtpHost
	s.SmtpPort = c.SmtpPort
	s.Stun = c.Stun
	s.StunServers = c.StunServers
	s.FrontendURL = c.FrontendURL
	s.JwtValidityDuration = c.JwtValidityDuration
	s.AllowedEmailDomains = c.AllowedEmailDomains
	s.LicenseValue = c.LicenseValue
	s.NetmakerTenantID = c.NetmakerTenantID
	s.ManageDNS = c.ManageDNS
	s.MetricInterval = c.MetricInterval
	s.MetricsPort = c.MetricsPort
	s.NetclientAutoUpdate = c.NetclientAutoUpdate
	s.Telemetry = c.Telemetry
	s.RacAutoDisable = c.RacAutoDisable
	s.RacRestrictToSingleNetwork = c.RacRestrictToSingleNetwork
}

func setServerInfo() {
	ServerInfo = GetServerInfo()
}

// GetServerInfo - gets the server config into memory from file or env
func GetServerInfo() models.ServerConfig {
	var cfg models.ServerConfig
	serverSettings := GetServerSettings()
	cfg.Server = servercfg.GetServer()
	if servercfg.GetBrokerType() == servercfg.EmqxBrokerType {
		cfg.MQUserName = "HOST_ID"
		cfg.MQPassword = "HOST_PASS"
	} else {
		cfg.MQUserName = servercfg.GetMqUserName()
		cfg.MQPassword = servercfg.GetMqPassword()
	}
	cfg.API = servercfg.GetAPIConnString()
	cfg.CoreDNSAddr = servercfg.GetCoreDNSAddr()
	cfg.APIPort = servercfg.GetAPIPort()
	cfg.DNSMode = "off"
	cfg.Broker = servercfg.GetPublicBrokerEndpoint()
	cfg.BrokerType = servercfg.GetBrokerType()
	if servercfg.IsDNSMode() {
		cfg.DNSMode = "on"
	}
	cfg.Version = servercfg.GetVersion()
	cfg.IsPro = servercfg.IsPro
	cfg.MetricInterval = serverSettings.MetricInterval
	cfg.MetricsPort = serverSettings.MetricsPort
	cfg.ManageDNS = serverSettings.ManageDNS
	cfg.Stun = serverSettings.Stun
	cfg.StunServers = serverSettings.StunServers
	cfg.DefaultDomain = servercfg.GetDefaultDomain()
	cfg.EndpointDetection = serverSettings.EndpointDetection
	return cfg
}
