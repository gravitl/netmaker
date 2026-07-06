package logic

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitl/netmaker/config"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
	"gorm.io/datatypes"
)

var (
	ErrInvalidJwtValidityDuration = errors.New("invalid jwt validity duration")
	ErrFlowLogsNotSupported       = errors.New("flow logs not supported")
	ErrInvalidIPDetectionInterval = errors.New("invalid ip detection interval (must be greater than or equal to 15s)")
)

var ServerSettingsDBKey = "server_cfg" // kept for migration reference only
var SettingsMutex = &sync.RWMutex{}

var serverSettingsCache atomic.Value

var defaultUserSettings = models.UserSettings{
	TextSize:      "16",
	Theme:         models.Dark,
	ReducedMotion: false,
}

func GetServerSettings() (s models.ServerSettings) {
	if cached, ok := serverSettingsCache.Load().(*models.ServerSettings); ok && cached != nil {
		return *cached
	}
	s, err := getServerSettingsFromDB()
	if err == nil {
		serverSettingsCache.Store(&s)
	}
	return
}

// InvalidateServerSettingsCache clears the in-memory settings cache so
// the next GetServerSettings call re-reads from the database.
func InvalidateServerSettingsCache() {
	serverSettingsCache.Store((*models.ServerSettings)(nil))
}

func getServerSettingsFromDB() (models.ServerSettings, error) {
	// TODO: replace with tenant ID from context once multi-tenancy is fully wired
	defaultTenant := &schema.Tenant{}
	err := defaultTenant.GetDefault(db.WithContext(context.TODO()))
	if err != nil {
		return models.ServerSettings{}, err
	}

	settingsRecord := &schema.TenantSettingsRecord{Key: defaultTenant.ID}
	err = settingsRecord.Get(db.WithContext(context.TODO()))
	if err != nil {
		return models.ServerSettings{}, err
	}
	return settingsRecord.Value.Data(), nil
}

func UpsertServerSettings(s models.ServerSettings) error {
	// get curr settings from DB directly (not cache) for accurate comparison
	currSettings, _ := getServerSettingsFromDB()
	if s.ClientSecret == Mask() {
		s.ClientSecret = currSettings.ClientSecret
	}
	if s.OktaAPIToken == Mask() {
		s.OktaAPIToken = currSettings.OktaAPIToken
	}
	if s.EmailSenderPassword == Mask() {
		s.EmailSenderPassword = currSettings.EmailSenderPassword
	}

	if servercfg.DeployedByOperator() {
		s.BasicAuth = true
	}

	var userFilters []string
	for _, userFilter := range s.UserFilters {
		userFilter = strings.TrimSpace(userFilter)
		if userFilter != "" {
			userFilters = append(userFilters, userFilter)
		}
	}
	s.UserFilters = userFilters

	var groupFilters []string
	for _, groupFilter := range s.GroupFilters {
		groupFilter = strings.TrimSpace(groupFilter)
		if groupFilter != "" {
			groupFilters = append(groupFilters, groupFilter)
		}
	}
	s.GroupFilters = groupFilters
	// TODO: replace with tenant ID from context once multi-tenancy is fully wired
	defaultTenant := &schema.Tenant{}
	err := defaultTenant.GetDefault(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}

	settingsRecord := &schema.TenantSettingsRecord{Key: defaultTenant.ID, Value: datatypes.NewJSONType(s)}
	err = settingsRecord.Upsert(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}
	serverSettingsCache.Store(&s)
	if PublishServerSync != nil {
		PublishServerSync(SyncTypeSettings)
	}
	return nil
}

func GetUserSettings(username string) models.UserSettings {
	user := schema.User{Username: username}
	err := user.Get(db.WithContext(context.TODO()))
	if err != nil {
		return defaultUserSettings
	}
	return models.UserSettings{
		Theme:         user.Theme,
		TextSize:      user.TextSize,
		ReducedMotion: user.ReducedMotion,
	}
}

func UpsertUserSettings(username string, userSettings models.UserSettings) error {
	if userSettings.TextSize == "" {
		userSettings.TextSize = "16"
	}
	if userSettings.Theme == "" {
		userSettings.Theme = models.Dark
	}
	u := schema.User{
		Username:      username,
		Theme:         userSettings.Theme,
		TextSize:      userSettings.TextSize,
		ReducedMotion: userSettings.ReducedMotion,
	}
	return u.UpdateUserSettings(db.WithContext(context.TODO()))
}

func ValidateNewSettings(req models.ServerSettings) error {
	// TODO: add checks for different fields
	if req.JwtValidityDuration > 525600 || req.JwtValidityDuration < 5 {
		return ErrInvalidJwtValidityDuration
	}

	if req.EnableFlowLogs && !GetFeatureFlags().EnableFlowLogs {
		return ErrFlowLogsNotSupported
	}

	if req.IPDetectionInterval < 15 {
		return ErrInvalidIPDetectionInterval
	}

	return nil
}

func GetServerSettingsFromEnv() (s models.ServerSettings) {

	s = models.ServerSettings{
		NetclientAutoUpdate: servercfg.AutoUpdateEnabled(),
		Verbosity:           servercfg.GetVerbosity(),
		AuthProvider:        os.Getenv("AUTH_PROVIDER"),
		OIDCIssuer:          os.Getenv("OIDC_ISSUER"),
		ClientID:            os.Getenv("CLIENT_ID"),
		ClientSecret:        os.Getenv("CLIENT_SECRET"),
		AzureTenant:         servercfg.GetAzureTenant(),
		Telemetry:           servercfg.Telemetry(),
		BasicAuth:           servercfg.IsBasicAuthEnabled(),
		JwtValidityDuration: servercfg.GetJwtValidityDurationFromEnv() / 60,
		// setting client's jwt validity duration to be the same as that of
		// dashboard.
		JwtValidityDurationClients: servercfg.GetJwtValidityDurationFromEnv() / 60,
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
	settings := GetServerSettings()
	cfg.APIConnString = servercfg.GetAPIConnString()
	cfg.CoreDNSAddr = servercfg.GetCoreDNSAddr()
	cfg.APIHost = servercfg.GetAPIHost()
	cfg.APIPort = servercfg.GetAPIPort()
	cfg.MasterKey = "(hidden)"
	cfg.DNSKey = "(hidden)"
	cfg.AllowedOrigin = servercfg.GetAllowedOrigin()
	cfg.RestBackend = "off"
	cfg.HostName = servercfg.GetHostName()
	cfg.BrokerType = servercfg.GetBrokerType()
	cfg.EmqxRestEndpoint = servercfg.GetEmqxRestEndpoint()
	if settings.NetclientAutoUpdate {
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
	var authInfo = GetAuthProviderInfo(settings)
	cfg.AuthProvider = authInfo[0]
	cfg.ClientID = authInfo[1]
	cfg.ClientSecret = authInfo[2]
	cfg.FrontendURL = servercfg.GetFrontendURL()
	cfg.AzureTenant = settings.AzureTenant
	cfg.Telemetry = settings.Telemetry
	cfg.Server = servercfg.GetServer()
	cfg.Verbosity = settings.Verbosity
	cfg.IsPro = "no"
	if servercfg.IsPro {
		cfg.IsPro = "yes"
	}
	cfg.JwtValidityDuration = time.Duration(settings.JwtValidityDuration) * time.Minute
	cfg.JwtValidityDurationClients = time.Duration(settings.JwtValidityDurationClients) * time.Minute
	cfg.RacRestrictToSingleNetwork = settings.RacRestrictToSingleNetwork
	cfg.MetricInterval = settings.MetricInterval
	cfg.MetricsPort = settings.MetricsPort
	cfg.ManageDNS = settings.ManageDNS
	cfg.Stun = settings.Stun
	cfg.StunServers = settings.StunServers
	cfg.DefaultDomain = settings.DefaultDomain
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
	cfg.GRPC = fmt.Sprintf("grpc.%s", servercfg.GetNmBaseDomain())
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
	cfg.IPDetectionInterval = serverSettings.IPDetectionInterval
	cfg.ManageDNS = serverSettings.ManageDNS
	cfg.Stun = serverSettings.Stun
	cfg.StunServers = serverSettings.StunServers
	cfg.DefaultDomain = serverSettings.DefaultDomain
	cfg.EndpointDetection = serverSettings.EndpointDetection
	cfg.PeerConnectionCheckInterval = serverSettings.PeerConnectionCheckInterval
	key, _ := RetrievePublicTrafficKey()
	cfg.TrafficKey = key
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

// GetJwtValidityDuration - returns the JWT validity duration in minutes
func GetJwtValidityDuration() time.Duration {
	return time.Duration(GetServerSettings().JwtValidityDuration) * time.Minute
}

// GetJwtValidityDurationForClients returns the JWT validity duration in
// minutes for clients.
func GetJwtValidityDurationForClients() time.Duration {
	return time.Duration(GetServerSettings().JwtValidityDurationClients) * time.Minute
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

func SmtpSkipTlsVerify() bool {
	return GetServerSettings().SmtpSkipTlsVerify
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
func GetAuthProviderInfo(settings models.ServerSettings) (pi []string) {
	var authProvider = ""

	defer func() {
		if authProvider == "okta" || authProvider == "oidc" {
			if settings.OIDCIssuer != "" {
				pi = append(pi, settings.OIDCIssuer)
			} else {
				pi = []string{"", "", ""}
			}
		}
	}()

	if settings.AuthProvider != "" && settings.ClientID != "" && settings.ClientSecret != "" {
		authProvider = strings.ToLower(settings.AuthProvider)
		if authProvider == "google" || authProvider == "azure-ad" || authProvider == "github" || authProvider == "okta" || authProvider == "oidc" {
			return []string{authProvider, settings.ClientID, settings.ClientSecret}
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

// IsSyncEnabled returns whether auth provider sync is enabled.
func IsSyncEnabled() bool {
	return GetServerSettings().SyncEnabled
}

// GetIDPSyncInterval returns the interval at which the netmaker should sync
// data from IDP.
func GetIDPSyncInterval() time.Duration {
	syncInterval, err := time.ParseDuration(GetServerSettings().IDPSyncInterval)
	if err != nil {
		return 24 * time.Hour
	}

	if syncInterval == 0 {
		return 24 * time.Hour
	}

	return syncInterval
}

// GetMetricsPort - get metrics port
func GetMetricsPort() int {
	return GetServerSettings().MetricsPort
}

// GetMetricIntervalInMinutes returns the publish-to-exporter interval from server
// settings (dashboard), with fallback to servercfg / env when unset or invalid.
func GetMetricIntervalInMinutes() time.Duration {
	mi := strings.TrimSpace(GetServerSettings().MetricInterval)
	if mi != "" {
		if interval, err := strconv.Atoi(mi); err == nil && interval > 0 {
			return time.Duration(interval) * time.Minute
		}
	}
	return servercfg.GetMetricIntervalInMinutes()
}

var (
	metricExportIntervalMu   sync.Mutex
	metricExportIntervalSubs []chan struct{}
)

// SubscribeMetricExportIntervalReset returns a channel notified when the metric interval setting changes.
func SubscribeMetricExportIntervalReset() <-chan struct{} {
	ch := make(chan struct{}, 1)
	metricExportIntervalMu.Lock()
	metricExportIntervalSubs = append(metricExportIntervalSubs, ch)
	metricExportIntervalMu.Unlock()
	return ch
}

// NotifyMetricExportIntervalChanged signals mq.Keepalive to reset the metrics export ticker.
func NotifyMetricExportIntervalChanged() {
	metricExportIntervalMu.Lock()
	defer metricExportIntervalMu.Unlock()
	for _, ch := range metricExportIntervalSubs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
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
	if servercfg.DeployedByOperator() {
		return true
	}

	return GetServerSettings().BasicAuth
}

// IsMFAEnforced returns whether MFA has been enforced.
func IsMFAEnforced() bool {
	return GetServerSettings().MFAEnforced
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

func Mask() string {
	return ("..................")
}
