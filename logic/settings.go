package logic

import (
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/config"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

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
	return nil
}

func ValidateNewSettings(req models.ServerSettings) bool {
	return true
}

func GetServerSettingsFromEnv() (s models.ServerSettings) {

	s = models.ServerSettings{
		NetclientAutoUpdate:        servercfg.AutoUpdateEnabled(),
		Verbosity:                  servercfg.GetVerbosity(),
		AuthProvider:               os.Getenv("AUTH_PROVIDER"),
		OIDCIssuer:                 os.Getenv("OIDC_ISSUER"),
		ClientID:                   os.Getenv("CLIENT_ID"),
		ClientSecret:               os.Getenv("CLIENT_SECRET"),
		AzureTenant:                servercfg.GetAzureTenant(),
		Telemetry:                  servercfg.Telemetry(),
		BasicAuth:                  servercfg.IsBasicAuthEnabled(),
		JwtValidityDuration:        servercfg.GetJwtValidityDuration(),
		RacAutoDisable:             servercfg.GetRacAutoDisable(),
		RacRestrictToSingleNetwork: servercfg.GetRacRestrictToSingleNetwork(),
		EndpointDetection:          servercfg.IsEndpointDetectionEnabled(),
		AllowedEmailDomains:        servercfg.GetAllowedEmailDomains(),
		EmailSenderAddr:            servercfg.GetSenderEmail(),
		EmailSenderUser:            servercfg.GetSenderUser(),
		EmailSenderPassword:        servercfg.GetEmaiSenderPassword(),
		SmtpHost:                   servercfg.GetSmtpHost(),
		SmtpPort:                   servercfg.GetSmtpPort(),
		MetricInterval:             servercfg.GetMetricInterval(),
		MetricsPort:                servercfg.GetMetricsPort(),
		ManageDNS:                  servercfg.GetManageDNS(),
		DefaultDomain:              servercfg.GetDefaultDomain(),
		Stun:                       servercfg.IsStunEnabled(),
		StunServers:                servercfg.GetStunServers(),
	}

	return
}

// GetServerConfig - gets the server config into memory from file or env
func GetServerConfig() config.ServerConfig {
	var cfg config.ServerConfig
	cfg.APIConnString = servercfg.GetAPIConnString()
	cfg.CoreDNSAddr = servercfg.GetCoreDNSAddr()
	cfg.APIHost = servercfg.GetAPIHost()
	cfg.APIPort = servercfg.GetAPIPort()
	cfg.MasterKey = "(hidden)"
	cfg.DNSKey = "(hidden)"
	cfg.AllowedOrigin = servercfg.GetAllowedOrigin()
	cfg.RestBackend = "off"
	cfg.NodeID = servercfg.GetNodeID()
	cfg.BrokerType = servercfg.GetBrokerType()
	cfg.EmqxRestEndpoint = servercfg.GetEmqxRestEndpoint()
	if AutoUpdateEnabled() {
		cfg.NetclientAutoUpdate = "enabled"
	} else {
		cfg.NetclientAutoUpdate = "disabled"
	}
	if servercfg.IsRestBackend() {
		cfg.RestBackend = "on"
	}
	cfg.DNSMode = "off"
	if servercfg.IsDNSMode() {
		cfg.DNSMode = "on"
	}
	cfg.DisplayKeys = "off"
	if servercfg.IsDisplayKeys() {
		cfg.DisplayKeys = "on"
	}
	cfg.DisableRemoteIPCheck = "off"
	if servercfg.DisableRemoteIPCheck() {
		cfg.DisableRemoteIPCheck = "on"
	}
	cfg.Database = servercfg.GetDB()
	cfg.Platform = servercfg.GetPlatform()
	cfg.Version = servercfg.GetVersion()
	cfg.PublicIp = servercfg.GetServerHostIP()

	// == auth config ==
	var authInfo = GetAuthProviderInfo()
	cfg.AuthProvider = authInfo[0]
	cfg.ClientID = authInfo[1]
	cfg.ClientSecret = authInfo[2]
	cfg.FrontendURL = servercfg.GetFrontendURL()
	cfg.AzureTenant = GetAzureTenant()
	cfg.Telemetry = Telemetry()
	cfg.Server = servercfg.GetServer()
	cfg.Verbosity = GetVerbosity()
	cfg.IsPro = "no"
	if servercfg.IsPro {
		cfg.IsPro = "yes"
	}
	cfg.JwtValidityDuration = GetJwtValidityDuration()
	cfg.RacAutoDisable = GetRacAutoDisable()
	cfg.RacRestrictToSingleNetwork = GetRacRestrictToSingleNetwork()
	cfg.MetricInterval = GetMetricInterval()
	cfg.ManageDNS = GetManageDNS()
	cfg.Stun = IsStunEnabled()
	cfg.StunServers = GetStunServers()
	cfg.DefaultDomain = GetDefaultDomain()
	return cfg
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
	cfg.DefaultDomain = serverSettings.DefaultDomain
	cfg.EndpointDetection = serverSettings.EndpointDetection
	return cfg
}

// GetDefaultDomain - get the default domain
func GetDefaultDomain() string {
	return GetServerSettings().DefaultDomain
}

func ValidateDomain(domain string) bool {
	domainPattern := `[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}(\.[a-zA-Z0-9][a-zA-Z0-9_-]{0,62})*(\.[a-zA-Z][a-zA-Z0-9]{0,10}){1}`

	exp := regexp.MustCompile("^" + domainPattern + "$")

	return exp.MatchString(domain)
}

// Telemetry - checks if telemetry data should be sent
func Telemetry() string {
	return GetServerSettings().Telemetry
}

// GetJwtValidityDuration - returns the JWT validity duration in seconds
func GetJwtValidityDuration() time.Duration {
	return GetServerConfig().JwtValidityDuration
}

// GetRacAutoDisable - returns whether the feature to autodisable RAC is enabled
func GetRacAutoDisable() bool {
	return GetServerSettings().RacAutoDisable
}

// GetRacRestrictToSingleNetwork - returns whether the feature to allow simultaneous network connections via RAC is enabled
func GetRacRestrictToSingleNetwork() bool {
	return GetServerSettings().RacRestrictToSingleNetwork
}

func GetSmtpHost() string {
	return GetServerSettings().SmtpHost
}

func GetSmtpPort() int {
	return GetServerSettings().SmtpPort
}

func GetSenderEmail() string {
	return GetServerSettings().EmailSenderAddr
}

func GetSenderUser() string {
	return GetServerSettings().EmailSenderUser
}

func GetEmaiSenderPassword() string {
	return GetServerSettings().EmailSenderPassword
}

// AutoUpdateEnabled returns a boolean indicating whether netclient auto update is enabled or disabled
// default is enabled
func AutoUpdateEnabled() bool {
	return GetServerSettings().NetclientAutoUpdate
}

// GetAuthProviderInfo = gets the oauth provider info
func GetAuthProviderInfo() (pi []string) {
	var authProvider = ""

	defer func() {
		if authProvider == "oidc" {
			if GetServerSettings().OIDCIssuer != "" {
				pi = append(pi, GetServerSettings().OIDCIssuer)
			} else {
				pi = []string{"", "", ""}
			}
		}
	}()

	if GetServerSettings().AuthProvider != "" && GetServerSettings().ClientID != "" && GetServerSettings().ClientSecret != "" {
		authProvider = strings.ToLower(GetServerSettings().AuthProvider)
		if authProvider == "google" || authProvider == "azure-ad" || authProvider == "github" || authProvider == "oidc" {
			return []string{authProvider, GetServerSettings().ClientID, GetServerSettings().ClientSecret}
		} else {
			authProvider = ""
		}
	}
	return []string{"", "", ""}
}

// GetAzureTenant - retrieve the azure tenant ID from env variable or config file
func GetAzureTenant() string {
	return GetServerSettings().AzureTenant
}

// GetMetricsPort - get metrics port
func GetMetricsPort() int {
	return GetServerSettings().MetricsPort
}

// GetMetricInterval - get the publish metric interval
func GetMetricIntervalInMinutes() time.Duration {
	//default 15 minutes
	mi := "15"
	if os.Getenv("PUBLISH_METRIC_INTERVAL") != "" {
		mi = os.Getenv("PUBLISH_METRIC_INTERVAL")
	}
	interval, err := strconv.Atoi(mi)
	if err != nil {
		interval = 15
	}

	return time.Duration(interval) * time.Minute
}

// GetMetricInterval - get the publish metric interval
func GetMetricInterval() string {
	return GetServerSettings().MetricInterval
}

// GetManageDNS - if manage DNS enabled or not
func GetManageDNS() bool {
	return GetServerSettings().ManageDNS
}

// IsBasicAuthEnabled - checks if basic auth has been configured to be turned off
func IsBasicAuthEnabled() bool {
	return GetServerSettings().BasicAuth
}

// IsEndpointDetectionEnabled - returns true if endpoint detection enabled
func IsEndpointDetectionEnabled() bool {
	return GetServerSettings().EndpointDetection
}

// IsStunEnabled - returns true if STUN set to on
func IsStunEnabled() bool {
	return GetServerSettings().Stun
}

func GetStunServers() string {
	return GetServerSettings().StunServers
}

// GetAllowedEmailDomains - gets the allowed email domains for oauth signup
func GetAllowedEmailDomains() string {
	return GetServerSettings().AllowedEmailDomains
}

func GetVerbosity() int32 {
	return GetServerSettings().Verbosity
}
