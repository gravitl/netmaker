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
	"time"

	"github.com/gravitl/netmaker/config"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/gravitl/netmaker/servercfg"
	"gorm.io/datatypes"
)

var (
	ErrInvalidJwtValidityDuration = errors.New("invalid jwt validity duration")
	ErrFlowLogsNotSupported       = errors.New("flow logs not supported")
	ErrInvalidIPDetectionInterval = errors.New("invalid ip detection interval (must be greater than or equal to 15s)")
)

var SettingsMutex = &sync.RWMutex{}

var serverSettingsCache sync.Map

var defaultUserSettings = models.UserSettings{
	TextSize:      "16",
	Theme:         models.Dark,
	ReducedMotion: false,
}

func GetServerSettings(ctx context.Context) (s models.ServerSettings) {
	tenantID := scope.ID(ctx)
	if cached, ok := serverSettingsCache.Load(tenantID); ok {
		if cachedSettings, ok := cached.(*models.ServerSettings); ok && cachedSettings != nil {
			return *cachedSettings
		}
	}
	s, err := getServerSettingsFromDB(ctx)
	if err == nil {
		serverSettingsCache.Store(tenantID, &s)
	}
	return
}

// InvalidateServerSettingsCache clears the in-memory settings cache for the tenant
// scoped by ctx so the next GetServerSettings call re-reads from the database.
func InvalidateServerSettingsCache(ctx context.Context) {
	serverSettingsCache.Delete(scope.ID(ctx))
}

func getServerSettingsFromDB(ctx context.Context) (models.ServerSettings, error) {
	settingsRecord := &schema.TenantSettingsRecord{Key: scope.ID(ctx)}
	err := settingsRecord.Get(ctx)
	if err != nil {
		return models.ServerSettings{}, err
	}
	return settingsRecord.Value.Data(), nil
}

func UpsertServerSettings(ctx context.Context, s models.ServerSettings) error {
	// get curr settings from DB directly (not cache) for accurate comparison
	currSettings, _ := getServerSettingsFromDB(ctx)
	if s.ClientSecret == Mask() {
		s.ClientSecret = currSettings.ClientSecret
	}
	if s.OktaAPIToken == Mask() {
		s.OktaAPIToken = currSettings.OktaAPIToken
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

	settingsRecord := &schema.TenantSettingsRecord{Key: scope.ID(ctx), Value: datatypes.NewJSONType(s)}
	err := settingsRecord.Upsert(ctx)
	if err != nil {
		return err
	}
	serverSettingsCache.Store(scope.ID(ctx), &s)
	if PublishServerSync != nil {
		PublishServerSync(ctx, SyncTypeSettings)
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
		MetricInterval:             servercfg.GetMetricInterval(),
		MetricsPort:                servercfg.GetMetricsPort(),
		ManageDNS:                  servercfg.GetManageDNS(),
		DefaultDomain:              servercfg.GetDefaultDomain(),
		Stun:                       servercfg.IsStunEnabled(),
		StunServers:                servercfg.GetStunServers(),
	}

	return
}

// GetOrgSettingsFromEnv returns organization settings seeded from env/config
// fallbacks (used to populate defaults for orgs with no settings record yet).
func GetOrgSettingsFromEnv() (s schema.OrganizationSettingsData) {
	s = schema.OrganizationSettingsData{
		EmailSenderAddr:     servercfg.GetSenderEmail(),
		EmailSenderUser:     servercfg.GetSenderUser(),
		EmailSenderPassword: servercfg.GetEmaiSenderPassword(),
		SmtpHost:            servercfg.GetSmtpHost(),
		SmtpPort:            servercfg.GetSmtpPort(),
	}
	return
}

// GetServerConfig - gets the server config into memory from file or env
func GetServerConfig(ctx context.Context) config.ServerConfig {
	var cfg config.ServerConfig
	settings := GetServerSettings(ctx)
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
func GetServerInfo(ctx context.Context) models.ServerConfig {
	var cfg models.ServerConfig
	serverSettings := GetServerSettings(ctx)
	cfg.TenantID = scope.ID(ctx)
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
func GetDefaultDomain(ctx context.Context) string {
	return GetServerSettings(ctx).DefaultDomain
}

func ValidateDomain(domain string) bool {
	domainPattern := `[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}(\.[a-zA-Z0-9][a-zA-Z0-9_-]{0,62})*(\.[a-zA-Z][a-zA-Z0-9]{0,10}){1}`

	exp := regexp.MustCompile("^" + domainPattern + "$")

	return exp.MatchString(domain)
}

// Telemetry - checks if telemetry data should be sent
func Telemetry(ctx context.Context) string {
	return GetServerSettings(ctx).Telemetry
}

// GetJwtValidityDuration - returns the JWT validity duration in minutes
func GetJwtValidityDuration(ctx context.Context) time.Duration {
	return time.Duration(GetServerSettings(ctx).JwtValidityDuration) * time.Minute
}

// GetJwtValidityDurationForClients returns the JWT validity duration in
// minutes for clients.
func GetJwtValidityDurationForClients(ctx context.Context) time.Duration {
	return time.Duration(GetServerSettings(ctx).JwtValidityDurationClients) * time.Minute
}

// GetRacRestrictToSingleNetwork - returns whether the feature to allow simultaneous network connections via RAC is enabled
func GetRacRestrictToSingleNetwork(ctx context.Context) bool {
	return GetServerSettings(ctx).RacRestrictToSingleNetwork
}

// GetOrgSettings returns the organization settings for the organization
// scoped by ctx (resolving through the tenant if ctx is tenant-scoped).
func GetOrgSettings(ctx context.Context) schema.OrganizationSettingsData {
	orgSettings := &schema.OrganizationSettings{
		ID: scope.ID(ctx),
	}
	err := orgSettings.Get(ctx)
	if err != nil {
		return schema.OrganizationSettingsData{}
	}

	return orgSettings.Settings.Data()
}

func GetSmtpHost(ctx context.Context) string {
	return GetOrgSettings(ctx).SmtpHost
}

func GetSmtpPort(ctx context.Context) int {
	return GetOrgSettings(ctx).SmtpPort
}

func SmtpSkipTlsVerify(ctx context.Context) bool {
	return GetOrgSettings(ctx).SmtpSkipTlsVerify
}

func GetSenderEmail(ctx context.Context) string {
	return GetOrgSettings(ctx).EmailSenderAddr
}

func GetSenderUser(ctx context.Context) string {
	return GetOrgSettings(ctx).EmailSenderUser
}

func GetEmaiSenderPassword(ctx context.Context) string {
	return GetOrgSettings(ctx).EmailSenderPassword
}

// AutoUpdateEnabled returns a boolean indicating whether netclient auto update is enabled or disabled
// default is enabled
func AutoUpdateEnabled(ctx context.Context) bool {
	return GetServerSettings(ctx).NetclientAutoUpdate
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
func GetAzureTenant(ctx context.Context) string {
	return GetServerSettings(ctx).AzureTenant
}

// IsSyncEnabled returns whether auth provider sync is enabled.
func IsSyncEnabled(ctx context.Context) bool {
	return GetServerSettings(ctx).SyncEnabled
}

// GetIDPSyncInterval returns the interval at which the netmaker should sync
// data from IDP.
func GetIDPSyncInterval(ctx context.Context) time.Duration {
	syncInterval, err := time.ParseDuration(GetServerSettings(ctx).IDPSyncInterval)
	if err != nil {
		return 24 * time.Hour
	}

	if syncInterval == 0 {
		return 24 * time.Hour
	}

	return syncInterval
}

// GetMetricsPort - get metrics port
func GetMetricsPort(ctx context.Context) int {
	return GetServerSettings(ctx).MetricsPort
}

// GetMetricIntervalInMinutes returns the publish-to-exporter interval from server
// settings (dashboard), with fallback to servercfg / env when unset or invalid.
func GetMetricIntervalInMinutes(ctx context.Context) time.Duration {
	mi := strings.TrimSpace(GetServerSettings(ctx).MetricInterval)
	if mi != "" {
		if interval, err := strconv.Atoi(mi); err == nil && interval > 0 {
			return time.Duration(interval) * time.Minute
		}
	}
	return servercfg.GetMetricIntervalInMinutes()
}

var (
	metricExportIntervalMu   sync.Mutex
	metricExportIntervalSubs = map[string][]chan struct{}{}
)

// SubscribeMetricExportIntervalReset returns a channel notified when the metric interval
// setting changes for ctx's tenant.
func SubscribeMetricExportIntervalReset(ctx context.Context) <-chan struct{} {
	tenantID := scope.ID(ctx)
	ch := make(chan struct{}, 1)
	metricExportIntervalMu.Lock()
	metricExportIntervalSubs[tenantID] = append(metricExportIntervalSubs[tenantID], ch)
	metricExportIntervalMu.Unlock()
	return ch
}

// NotifyMetricExportIntervalChanged signals mq.Keepalive to reset the metrics export ticker
// for ctx's tenant.
func NotifyMetricExportIntervalChanged(ctx context.Context) {
	tenantID := scope.ID(ctx)
	metricExportIntervalMu.Lock()
	defer metricExportIntervalMu.Unlock()
	for _, ch := range metricExportIntervalSubs[tenantID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// GetMetricInterval - get the publish metric interval
func GetMetricInterval(ctx context.Context) string {
	return GetServerSettings(ctx).MetricInterval
}

// GetManageDNS - if manage DNS enabled or not
func GetManageDNS(ctx context.Context) bool {
	return GetServerSettings(ctx).ManageDNS
}

// IsBasicAuthEnabled - checks if basic auth has been configured to be turned off
func IsBasicAuthEnabled(ctx context.Context) bool {
	if servercfg.DeployedByOperator() {
		return true
	}

	return GetServerSettings(ctx).BasicAuth
}

// IsMFAEnforced returns whether MFA has been enforced.
func IsMFAEnforced(ctx context.Context) bool {
	return GetServerSettings(ctx).MFAEnforced
}

// IsEndpointDetectionEnabled - returns true if endpoint detection enabled
func IsEndpointDetectionEnabled(ctx context.Context) bool {
	return GetServerSettings(ctx).EndpointDetection
}

// IsStunEnabled - returns true if STUN set to on
func IsStunEnabled(ctx context.Context) bool {
	return GetServerSettings(ctx).Stun
}

func GetStunServers(ctx context.Context) string {
	return GetServerSettings(ctx).StunServers
}

// GetAllowedEmailDomains - gets the allowed email domains for oauth signup
func GetAllowedEmailDomains(ctx context.Context) string {
	return GetServerSettings(ctx).AllowedEmailDomains
}

func GetVerbosity(ctx context.Context) int32 {
	return GetServerSettings(ctx).Verbosity
}

func Mask() string {
	return ("..................")
}
