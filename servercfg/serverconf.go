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

var (
	Version = "dev"
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
	cfg.MQPort = GetMQPort()
	cfg.MasterKey = "(hidden)"
	cfg.DNSKey = "(hidden)"
	cfg.AllowedOrigin = GetAllowedOrigin()
	cfg.RestBackend = "off"
	cfg.NodeID = GetNodeID()
	if IsRestBackend() {
		cfg.RestBackend = "on"
	}
	cfg.AgentBackend = "off"
	if IsAgentBackend() {
		cfg.AgentBackend = "on"
	}
	cfg.ClientMode = "off"
	if IsClientMode() != "off" {
		cfg.ClientMode = IsClientMode()
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
	if GetRce() {
		cfg.RCE = "on"
	} else {
		cfg.RCE = "off"
	}
	cfg.Telemetry = Telemetry()
	cfg.ManageIPTables = ManageIPTables()
	services := strings.Join(GetPortForwardServiceList(), ",")
	cfg.PortForwardServices = services
	cfg.Server = GetServer()
	cfg.Verbosity = GetVerbosity()

	return cfg
}

// GetServerConfig - gets the server config into memory from file or env
func GetServerInfo() models.ServerConfig {
	var cfg models.ServerConfig
	cfg.API = GetAPIConnString()
	cfg.CoreDNSAddr = GetCoreDNSAddr()
	cfg.APIPort = GetAPIPort()
	cfg.MQPort = GetMQPort()
	cfg.DNSMode = "off"
	if IsDNSMode() {
		cfg.DNSMode = "on"
	}
	cfg.Version = GetVersion()
	cfg.Server = GetServer()

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

// GetPodIP - get the pod's ip
func GetPodIP() string {
	podip := "127.0.0.1"
	if os.Getenv("POD_IP") != "" {
		podip = os.Getenv("POD_IP")
	}
	return podip
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

// GetDefaultNodeLimit - get node limit if one is set
func GetDefaultNodeLimit() int32 {
	var limit int32
	limit = 999999999
	envlimit, err := strconv.Atoi(os.Getenv("DEFAULT_NODE_LIMIT"))
	if err == nil && envlimit != 0 {
		limit = int32(envlimit)
	} else if config.Config.Server.DefaultNodeLimit != 0 {
		limit = config.Config.Server.DefaultNodeLimit
	}
	return limit
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

// GetMQPort - gets the mq port
func GetMQPort() string {
	port := "8883" //default
	if os.Getenv("MQ_PORT") != "" {
		port = os.Getenv("MQ_PORT")
	} else if config.Config.Server.MQPort != "" {
		port = config.Config.Server.MQPort
	}
	return port
}

// GetMessageQueueEndpoint - gets the message queue endpoint
func GetMessageQueueEndpoint() (string, bool) {
	host, _ := GetPublicIP()
	if os.Getenv("MQ_HOST") != "" {
		host = os.Getenv("MQ_HOST")
	} else if config.Config.Server.MQHOST != "" {
		host = config.Config.Server.MQHOST
	}
	secure := strings.Contains(host, "mqtts") || strings.Contains(host, "ssl")
	return host + ":" + GetMQServerPort(), secure
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

// GetDNSKey - gets the configured dns key of server
func GetDNSKey() string {
	key := "secretkey"
	if os.Getenv("DNS_KEY") != "" {
		key = os.Getenv("DNS_KEY")
	} else if config.Config.Server.DNSKey != "" {
		key = config.Config.Server.DNSKey
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

// IsAgentBackend - checks if agent backed is on or off
func IsAgentBackend() bool {
	isagent := true
	if os.Getenv("AGENT_BACKEND") != "" {
		if os.Getenv("AGENT_BACKEND") == "off" {
			isagent = false
		}
	} else if config.Config.Server.AgentBackend != "" {
		if config.Config.Server.AgentBackend == "off" {
			isagent = false
		}
	}
	return isagent
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

// IsClientMode - checks if it should run in client mode
func IsClientMode() string {
	isclient := "on"
	if os.Getenv("CLIENT_MODE") == "off" {
		isclient = "off"
	}
	if config.Config.Server.ClientMode == "off" {
		isclient = "off"
	}
	return isclient
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

// ManageIPTables - checks if iptables should be manipulated on host
func ManageIPTables() string {
	manage := "on"
	if os.Getenv("MANAGE_IPTABLES") == "off" {
		manage = "off"
	}
	if config.Config.Server.ManageIPTables == "off" {
		manage = "off"
	}
	return manage
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

	iplist := []string{"https://ip.server.gravitl.com", "https://ifconfig.me", "https://api.ipify.org", "https://ipinfo.io/ip"}
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
		platform = config.Config.Server.SQLConn
	}
	return platform
}

// GetIPForwardServiceList - get the list of services that the server should be forwarding
func GetPortForwardServiceList() []string {
	//services := "mq,dns,ssh"
	services := ""
	if os.Getenv("PORT_FORWARD_SERVICES") != "" {
		services = os.Getenv("PORT_FORWARD_SERVICES")
	} else if config.Config.Server.PortForwardServices != "" {
		services = config.Config.Server.PortForwardServices
	}
	serviceSlice := strings.Split(services, ",")
	return serviceSlice
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

// IsHostNetwork - checks if running on host network
func IsHostNetwork() bool {
	ishost := false
	if os.Getenv("HOST_NETWORK") == "on" {
		ishost = true
	} else if config.Config.Server.HostNetwork == "on" {
		ishost = true
	}
	return ishost
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

// GetServerCheckinInterval - gets the server check-in time
func GetServerCheckinInterval() int64 {
	var t = int64(5)
	var envt, _ = strconv.Atoi(os.Getenv("SERVER_CHECKIN_INTERVAL"))
	if envt > 0 {
		t = int64(envt)
	} else if config.Config.Server.ServerCheckinInterval > 0 {
		t = config.Config.Server.ServerCheckinInterval
	}
	return t
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

// GetRce - sees if Rce is enabled, off by default
func GetRce() bool {
	return os.Getenv("RCE") == "on" || config.Config.Server.RCE == "on"
}

// GetMQServerPort - get mq port for server
func GetMQServerPort() string {
	port := "1883" //default
	if os.Getenv("MQ_SERVER_PORT") != "" {
		port = os.Getenv("MQ_SERVER_PORT")
	} else if config.Config.Server.MQServerPort != "" {
		port = config.Config.Server.MQServerPort
	}
	return port
}
