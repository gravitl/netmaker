package servercfg

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gravitl/netmaker/config"

	"github.com/gravitl/netmaker/models"
)

// EmqxBrokerType denotes the broker type for EMQX MQTT
const EmqxBrokerType = "emqx"

// Emqxdeploy - emqx deploy type
type Emqxdeploy string

var (
	Version              = "dev"
	IsPro                = false
	ErrLicenseValidation error
	EmqxCloudDeploy      Emqxdeploy = "cloud"
	EmqxOnPremDeploy     Emqxdeploy = "on-prem"
)

// SetHost - sets the host ip
func SetHost() error {
	remoteip, err := GetPublicIP()
	if err != nil {
		return err
	}
	os.Setenv("SERVER_HOST", remoteip)
	return nil
}

// GetServerConfig - gets the server config into memory from file or env
func GetServerConfig() config.ServerConfig {
	var cfg config.ServerConfig
	cfg.APIConnString = GetAPIConnString()
	cfg.CoreDNSAddr = GetCoreDNSAddr()
	cfg.APIHost = GetAPIHost()
	cfg.APIPort = GetAPIPort()
	cfg.MasterKey = "(hidden)"
	cfg.DNSKey = "(hidden)"
	cfg.AllowedOrigin = GetAllowedOrigin()
	cfg.RestBackend = "off"
	cfg.NodeID = GetNodeID()
	cfg.BrokerType = GetBrokerType()
	cfg.EmqxRestEndpoint = GetEmqxRestEndpoint()
	if AutoUpdateEnabled() {
		cfg.NetclientAutoUpdate = "enabled"
	} else {
		cfg.NetclientAutoUpdate = "disabled"
	}
	if IsRestBackend() {
		cfg.RestBackend = "on"
	}
	cfg.DNSMode = "off"
	if IsDNSMode() {
		cfg.DNSMode = "on"
	}
	cfg.DisplayKeys = "off"
	if IsDisplayKeys() {
		cfg.DisplayKeys = "on"
	}
	cfg.DisableRemoteIPCheck = "off"
	if DisableRemoteIPCheck() {
		cfg.DisableRemoteIPCheck = "on"
	}
	cfg.Database = GetDB()
	cfg.Platform = GetPlatform()
	cfg.Version = GetVersion()

	// == auth config ==
	var authInfo = GetAuthProviderInfo()
	cfg.AuthProvider = authInfo[0]
	cfg.ClientID = authInfo[1]
	cfg.ClientSecret = authInfo[2]
	cfg.FrontendURL = GetFrontendURL()
	cfg.Telemetry = Telemetry()
	cfg.Server = GetServer()
	cfg.Verbosity = GetVerbosity()
	cfg.IsPro = "no"
	if IsPro {
		cfg.IsPro = "yes"
	}
	cfg.JwtValidityDuration = GetJwtValidityDuration()
	cfg.RacAutoDisable = GetRacAutoDisable()
	cfg.MetricInterval = GetMetricInterval()
	return cfg
}

// GetJwtValidityDuration - returns the JWT validity duration in seconds
func GetJwtValidityDuration() time.Duration {
	var defaultDuration = time.Duration(24) * time.Hour
	if os.Getenv("JWT_VALIDITY_DURATION") != "" {
		t, err := strconv.Atoi(os.Getenv("JWT_VALIDITY_DURATION"))
		if err != nil {
			return defaultDuration
		}
		return time.Duration(t) * time.Second
	}
	return defaultDuration
}

// GetRacAutoDisable - returns whether the feature to autodisable RAC is enabled
func GetRacAutoDisable() bool {
	return os.Getenv("RAC_AUTO_DISABLE") == "true"
}

// GetServerInfo - gets the server config into memory from file or env
func GetServerInfo() models.ServerConfig {
	var cfg models.ServerConfig
	cfg.Server = GetServer()
	if GetBrokerType() == EmqxBrokerType {
		cfg.MQUserName = "HOST_ID"
		cfg.MQPassword = "HOST_PASS"
	} else {
		cfg.MQUserName = GetMqUserName()
		cfg.MQPassword = GetMqPassword()
	}
	cfg.API = GetAPIConnString()
	cfg.CoreDNSAddr = GetCoreDNSAddr()
	cfg.APIPort = GetAPIPort()
	cfg.DNSMode = "off"
	cfg.Broker = GetPublicBrokerEndpoint()
	cfg.BrokerType = GetBrokerType()
	if IsDNSMode() {
		cfg.DNSMode = "on"
	}
	cfg.Version = GetVersion()
	cfg.IsPro = IsPro
	cfg.MetricInterval = GetMetricInterval()
	return cfg
}

// GetFrontendURL - gets the frontend url
func GetFrontendURL() string {
	var frontend = ""
	if os.Getenv("FRONTEND_URL") != "" {
		frontend = os.Getenv("FRONTEND_URL")
	} else if config.Config.Server.FrontendURL != "" {
		frontend = config.Config.Server.FrontendURL
	}
	return frontend
}

// GetAPIConnString - gets the api connections string
func GetAPIConnString() string {
	conn := ""
	if os.Getenv("SERVER_API_CONN_STRING") != "" {
		conn = os.Getenv("SERVER_API_CONN_STRING")
	} else if config.Config.Server.APIConnString != "" {
		conn = config.Config.Server.APIConnString
	}
	return conn
}

// SetVersion - set version of netmaker
func SetVersion(v string) {
	Version = v
}

// GetVersion - version of netmaker
func GetVersion() string {
	return Version
}

// GetDB - gets the database type
func GetDB() string {
	database := "sqlite"
	if os.Getenv("DATABASE") != "" {
		database = os.Getenv("DATABASE")
	} else if config.Config.Server.Database != "" {
		database = config.Config.Server.Database
	}
	return database
}

// CacheEnabled - checks if cache is enabled
func CacheEnabled() bool {
	caching := true
	if os.Getenv("CACHING_ENABLED") == "false" {
		caching = false
	} else if config.Config.Server.CacheEnabled == "false" {
		caching = false
	}
	return caching
}

// GetAPIHost - gets the api host
func GetAPIHost() string {
	serverhost := "127.0.0.1"
	remoteip, _ := GetPublicIP()
	if os.Getenv("SERVER_HTTP_HOST") != "" {
		serverhost = os.Getenv("SERVER_HTTP_HOST")
	} else if config.Config.Server.APIHost != "" {
		serverhost = config.Config.Server.APIHost
	} else if os.Getenv("SERVER_HOST") != "" {
		serverhost = os.Getenv("SERVER_HOST")
	} else {
		if remoteip != "" {
			serverhost = remoteip
		}
	}
	return serverhost
}

// GetAPIPort - gets the api port
func GetAPIPort() string {
	apiport := "8081"
	if os.Getenv("API_PORT") != "" {
		apiport = os.Getenv("API_PORT")
	} else if config.Config.Server.APIPort != "" {
		apiport = config.Config.Server.APIPort
	}
	return apiport
}

// GetCoreDNSAddr - gets the core dns address
func GetCoreDNSAddr() string {
	addr, _ := GetPublicIP()
	if os.Getenv("COREDNS_ADDR") != "" {
		addr = os.Getenv("COREDNS_ADDR")
	} else if config.Config.Server.CoreDNSAddr != "" {
		addr = config.Config.Server.CoreDNSAddr
	}
	return addr
}

// GetPublicBrokerEndpoint - returns the public broker endpoint which shall be used by netclient
func GetPublicBrokerEndpoint() string {
	if os.Getenv("BROKER_ENDPOINT") != "" {
		return os.Getenv("BROKER_ENDPOINT")
	} else {
		return config.Config.Server.Broker
	}
}

// GetOwnerEmail - gets the owner email (saas)
func GetOwnerEmail() string {
	return os.Getenv("SAAS_OWNER_EMAIL")
}

// GetMessageQueueEndpoint - gets the message queue endpoint
func GetMessageQueueEndpoint() (string, bool) {
	host, _ := GetPublicIP()
	if os.Getenv("SERVER_BROKER_ENDPOINT") != "" {
		host = os.Getenv("SERVER_BROKER_ENDPOINT")
	} else if config.Config.Server.ServerBrokerEndpoint != "" {
		host = config.Config.Server.ServerBrokerEndpoint
	} else if os.Getenv("BROKER_ENDPOINT") != "" {
		host = os.Getenv("BROKER_ENDPOINT")
	} else if config.Config.Server.Broker != "" {
		host = config.Config.Server.Broker
	} else {
		host += ":1883" // default
	}
	return host, strings.Contains(host, "wss") || strings.Contains(host, "ssl") || strings.Contains(host, "mqtts")
}

// GetBrokerType - returns the type of MQ broker
func GetBrokerType() string {
	if os.Getenv("BROKER_TYPE") != "" {
		return os.Getenv("BROKER_TYPE")
	} else {
		return "mosquitto"
	}
}

// GetMasterKey - gets the configured master key of server
func GetMasterKey() string {
	key := ""
	if os.Getenv("MASTER_KEY") != "" {
		key = os.Getenv("MASTER_KEY")
	} else if config.Config.Server.MasterKey != "" {
		key = config.Config.Server.MasterKey
	}
	return key
}

// GetAllowedOrigin - get the allowed origin
func GetAllowedOrigin() string {
	allowedorigin := "*"
	if os.Getenv("CORS_ALLOWED_ORIGIN") != "" {
		allowedorigin = os.Getenv("CORS_ALLOWED_ORIGIN")
	} else if config.Config.Server.AllowedOrigin != "" {
		allowedorigin = config.Config.Server.AllowedOrigin
	}
	return allowedorigin
}

// IsRestBackend - checks if rest is on or off
func IsRestBackend() bool {
	isrest := true
	if os.Getenv("REST_BACKEND") != "" {
		if os.Getenv("REST_BACKEND") == "off" {
			isrest = false
		}
	} else if config.Config.Server.RestBackend != "" {
		if config.Config.Server.RestBackend == "off" {
			isrest = false
		}
	}
	return isrest
}

// IsMetricsExporter - checks if metrics exporter is on or off
func IsMetricsExporter() bool {
	export := false
	if os.Getenv("METRICS_EXPORTER") != "" {
		if os.Getenv("METRICS_EXPORTER") == "on" {
			export = true
		}
	} else if config.Config.Server.MetricsExporter != "" {
		if config.Config.Server.MetricsExporter == "on" {
			export = true
		}
	}
	return export
}

// IsMessageQueueBackend - checks if message queue is on or off
func IsMessageQueueBackend() bool {
	ismessagequeue := true
	if os.Getenv("MESSAGEQUEUE_BACKEND") != "" {
		if os.Getenv("MESSAGEQUEUE_BACKEND") == "off" {
			ismessagequeue = false
		}
	} else if config.Config.Server.MessageQueueBackend != "" {
		if config.Config.Server.MessageQueueBackend == "off" {
			ismessagequeue = false
		}
	}
	return ismessagequeue
}

// Telemetry - checks if telemetry data should be sent
func Telemetry() string {
	telemetry := "on"
	if os.Getenv("TELEMETRY") == "off" {
		telemetry = "off"
	}
	if config.Config.Server.Telemetry == "off" {
		telemetry = "off"
	}
	return telemetry
}

// GetServer - gets the server name
func GetServer() string {
	server := ""
	if os.Getenv("SERVER_NAME") != "" {
		server = os.Getenv("SERVER_NAME")
	} else if config.Config.Server.Server != "" {
		server = config.Config.Server.Server
	}
	return server
}

func GetVerbosity() int32 {
	var verbosity = 0
	var err error
	if os.Getenv("VERBOSITY") != "" {
		verbosity, err = strconv.Atoi(os.Getenv("VERBOSITY"))
		if err != nil {
			verbosity = 0
		}
	} else if config.Config.Server.Verbosity != 0 {
		verbosity = int(config.Config.Server.Verbosity)
	}
	if verbosity < 0 || verbosity > 4 {
		verbosity = 0
	}
	return int32(verbosity)
}

// AutoUpdateEnabled returns a boolean indicating whether netclient auto update is enabled or disabled
// default is enabled
func AutoUpdateEnabled() bool {
	if os.Getenv("NETCLIENT_AUTO_UPDATE") == "disabled" {
		return false
	} else if config.Config.Server.NetclientAutoUpdate == "disabled" {
		return false
	}
	return true
}

// IsDNSMode - should it run with DNS
func IsDNSMode() bool {
	isdns := true
	if os.Getenv("DNS_MODE") != "" {
		if os.Getenv("DNS_MODE") == "off" {
			isdns = false
		}
	} else if config.Config.Server.DNSMode != "" {
		if config.Config.Server.DNSMode == "off" {
			isdns = false
		}
	}
	return isdns
}

// IsDisplayKeys - should server be able to display keys?
func IsDisplayKeys() bool {
	isdisplay := true
	if os.Getenv("DISPLAY_KEYS") != "" {
		if os.Getenv("DISPLAY_KEYS") == "off" {
			isdisplay = false
		}
	} else if config.Config.Server.DisplayKeys != "" {
		if config.Config.Server.DisplayKeys == "off" {
			isdisplay = false
		}
	}
	return isdisplay
}

// DisableRemoteIPCheck - disable the remote ip check
func DisableRemoteIPCheck() bool {
	disabled := false
	if os.Getenv("DISABLE_REMOTE_IP_CHECK") != "" {
		if os.Getenv("DISABLE_REMOTE_IP_CHECK") == "on" {
			disabled = true
		}
	} else if config.Config.Server.DisableRemoteIPCheck != "" {
		if config.Config.Server.DisableRemoteIPCheck == "on" {
			disabled = true
		}
	}
	return disabled
}

// GetPublicIP - gets public ip
func GetPublicIP() (string, error) {

	endpoint := ""
	var err error

	iplist := []string{"https://ifconfig.me", "https://api.ipify.org", "https://ipinfo.io/ip"}
	publicIpService := os.Getenv("PUBLIC_IP_SERVICE")
	if publicIpService != "" {
		// prepend the user-specified service so it's checked first
		iplist = append([]string{publicIpService}, iplist...)
	} else if config.Config.Server.PublicIPService != "" {
		publicIpService = config.Config.Server.PublicIPService

		// prepend the user-specified service so it's checked first
		iplist = append([]string{publicIpService}, iplist...)
	}

	for _, ipserver := range iplist {
		client := &http.Client{
			Timeout: time.Second * 10,
		}
		resp, err := client.Get(ipserver)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				continue
			}
			endpoint = string(bodyBytes)
			break
		}
	}
	if err == nil && endpoint == "" {
		err = errors.New("public address not found")
	}
	return endpoint, err
}

// GetPlatform - get the system type of server
func GetPlatform() string {
	platform := "linux"
	if os.Getenv("PLATFORM") != "" {
		platform = os.Getenv("PLATFORM")
	} else if config.Config.Server.Platform != "" {
		platform = config.Config.Server.Platform
	}
	return platform
}

// GetSQLConn - get the sql connection string
func GetSQLConn() string {
	sqlconn := "http://"
	if os.Getenv("SQL_CONN") != "" {
		sqlconn = os.Getenv("SQL_CONN")
	} else if config.Config.Server.SQLConn != "" {
		sqlconn = config.Config.Server.SQLConn
	}
	return sqlconn
}

// GetNodeID - gets the node id
func GetNodeID() string {
	var id string
	var err error
	// id = getMacAddr()
	if os.Getenv("NODE_ID") != "" {
		id = os.Getenv("NODE_ID")
	} else if config.Config.Server.NodeID != "" {
		id = config.Config.Server.NodeID
	} else {
		id, err = os.Hostname()
		if err != nil {
			return ""
		}
	}
	return id
}

func SetNodeID(id string) {
	config.Config.Server.NodeID = id
}

// GetAuthProviderInfo = gets the oauth provider info
func GetAuthProviderInfo() (pi []string) {
	var authProvider = ""

	defer func() {
		if authProvider == "oidc" {
			if os.Getenv("OIDC_ISSUER") != "" {
				pi = append(pi, os.Getenv("OIDC_ISSUER"))
			} else if config.Config.Server.OIDCIssuer != "" {
				pi = append(pi, config.Config.Server.OIDCIssuer)
			} else {
				pi = []string{"", "", ""}
			}
		}
	}()

	if os.Getenv("AUTH_PROVIDER") != "" && os.Getenv("CLIENT_ID") != "" && os.Getenv("CLIENT_SECRET") != "" {
		authProvider = strings.ToLower(os.Getenv("AUTH_PROVIDER"))
		if authProvider == "google" || authProvider == "azure-ad" || authProvider == "github" || authProvider == "oidc" {
			return []string{authProvider, os.Getenv("CLIENT_ID"), os.Getenv("CLIENT_SECRET")}
		} else {
			authProvider = ""
		}
	} else if config.Config.Server.AuthProvider != "" && config.Config.Server.ClientID != "" && config.Config.Server.ClientSecret != "" {
		authProvider = strings.ToLower(config.Config.Server.AuthProvider)
		if authProvider == "google" || authProvider == "azure-ad" || authProvider == "github" || authProvider == "oidc" {
			return []string{authProvider, config.Config.Server.ClientID, config.Config.Server.ClientSecret}
		}
	}
	return []string{"", "", ""}
}

// GetAzureTenant - retrieve the azure tenant ID from env variable or config file
func GetAzureTenant() string {
	var azureTenant = ""
	if os.Getenv("AZURE_TENANT") != "" {
		azureTenant = os.Getenv("AZURE_TENANT")
	} else if config.Config.Server.AzureTenant != "" {
		azureTenant = config.Config.Server.AzureTenant
	}
	return azureTenant
}

// GetMqPassword - fetches the MQ password
func GetMqPassword() string {
	password := ""
	if os.Getenv("MQ_PASSWORD") != "" {
		password = os.Getenv("MQ_PASSWORD")
	} else if config.Config.Server.MQPassword != "" {
		password = config.Config.Server.MQPassword
	}
	return password
}

// GetMqUserName - fetches the MQ username
func GetMqUserName() string {
	password := ""
	if os.Getenv("MQ_USERNAME") != "" {
		password = os.Getenv("MQ_USERNAME")
	} else if config.Config.Server.MQUserName != "" {
		password = config.Config.Server.MQUserName
	}
	return password
}

// GetMetricInterval - get the publish metric interval
func GetMetricInterval() string {
	//default 15 minutes
	mi := "15"
	if os.Getenv("metric_interval") != "" {
		mi = os.Getenv("metric_interval")
	}
	return mi
}

// GetEmqxRestEndpoint - returns the REST API Endpoint of EMQX
func GetEmqxRestEndpoint() string {
	return os.Getenv("EMQX_REST_ENDPOINT")
}

// IsBasicAuthEnabled - checks if basic auth has been configured to be turned off
func IsBasicAuthEnabled() bool {
	var enabled = true //default
	if os.Getenv("BASIC_AUTH") != "" {
		enabled = os.Getenv("BASIC_AUTH") == "yes"
	} else if config.Config.Server.BasicAuth != "" {
		enabled = config.Config.Server.BasicAuth == "yes"
	}
	return enabled
}

// GetLicenseKey - retrieves pro license value from env or conf files
func GetLicenseKey() string {
	licenseKeyValue := os.Getenv("LICENSE_KEY")
	if licenseKeyValue == "" {
		licenseKeyValue = config.Config.Server.LicenseValue
	}
	return licenseKeyValue
}

// GetNetmakerTenantID - get's the associated, Netmaker, tenant ID to verify ownership
func GetNetmakerTenantID() string {
	netmakerTenantID := os.Getenv("NETMAKER_TENANT_ID")
	if netmakerTenantID == "" {
		netmakerTenantID = config.Config.Server.NetmakerTenantID
	}
	return netmakerTenantID
}

// GetUserLimit - fetches free tier limits on users
func GetUserLimit() int {
	var userslimit int
	if os.Getenv("USERS_LIMIT") != "" {
		userslimit, _ = strconv.Atoi(os.Getenv("USERS_LIMIT"))
	} else {
		userslimit = config.Config.Server.UsersLimit
	}
	return userslimit
}

// GetNetworkLimit - fetches free tier limits on networks
func GetNetworkLimit() int {
	var networkslimit int
	if os.Getenv("NETWORKS_LIMIT") != "" {
		networkslimit, _ = strconv.Atoi(os.Getenv("NETWORKS_LIMIT"))
	} else {
		networkslimit = config.Config.Server.NetworksLimit
	}
	return networkslimit
}

// GetMachinesLimit - fetches free tier limits on machines (clients + hosts)
func GetMachinesLimit() int {
	if l, err := strconv.Atoi(os.Getenv("MACHINES_LIMIT")); err == nil {
		return l
	}
	return config.Config.Server.MachinesLimit
}

// GetIngressLimit - fetches free tier limits on ingresses
func GetIngressLimit() int {
	if l, err := strconv.Atoi(os.Getenv("INGRESSES_LIMIT")); err == nil {
		return l
	}
	return config.Config.Server.IngressesLimit
}

// GetEgressLimit - fetches free tier limits on egresses
func GetEgressLimit() int {
	if l, err := strconv.Atoi(os.Getenv("EGRESSES_LIMIT")); err == nil {
		return l
	}
	return config.Config.Server.EgressesLimit
}

// DeployedByOperator - returns true if the instance is deployed by netmaker operator
func DeployedByOperator() bool {
	if os.Getenv("DEPLOYED_BY_OPERATOR") != "" {
		return os.Getenv("DEPLOYED_BY_OPERATOR") == "true"
	}
	return config.Config.Server.DeployedByOperator
}

// IsEndpointDetectionEnabled - returns true if endpoint detection enabled
func IsEndpointDetectionEnabled() bool {
	var enabled = true //default
	if os.Getenv("ENDPOINT_DETECTION") != "" {
		enabled = os.Getenv("ENDPOINT_DETECTION") == "true"
	}
	return enabled
}

// GetEnvironment returns the environment the server is running in (e.g. dev, staging, prod...)
func GetEnvironment() string {
	if env := os.Getenv("ENVIRONMENT"); env != "" {
		return env
	}
	if env := config.Config.Server.Environment; env != "" {
		return env
	}
	return ""
}

// GetEmqxDeployType - fetches emqx deploy type this server uses
func GetEmqxDeployType() (deployType Emqxdeploy) {
	deployType = EmqxOnPremDeploy
	if os.Getenv("EMQX_DEPLOY_TYPE") == string(EmqxCloudDeploy) {
		deployType = EmqxCloudDeploy
	}
	return
}

// GetEmqxAppID - gets the emqx cloud app id
func GetEmqxAppID() string {
	return os.Getenv("EMQX_APP_ID")
}

// GetEmqxAppSecret - gets the emqx cloud app secret
func GetEmqxAppSecret() string {
	return os.Getenv("EMQX_APP_SECRET")
}

// GetAllowedEmailDomains - gets the allowed email domains for oauth signup
func GetAllowedEmailDomains() string {
	allowedDomains := "*"
	if os.Getenv("ALLOWED_EMAIL_DOMAINS") != "" {
		allowedDomains = os.Getenv("ALLOWED_EMAIL_DOMAINS")
	} else if config.Config.Server.AllowedEmailDomains != "" {
		allowedDomains = config.Config.Server.AllowedEmailDomains
	}
	return allowedDomains
}
