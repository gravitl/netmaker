package servercfg

import (
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gravitl/netmaker/config"
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
	cfg.GRPCConnString = GetGRPCConnString()
	cfg.GRPCHost = GetGRPCHost()
	cfg.GRPCPort = GetGRPCPort()
	cfg.MasterKey = "(hidden)"
	cfg.DNSKey = "(hidden)"
	cfg.AllowedOrigin = GetAllowedOrigin()
	cfg.RestBackend = "off"
	cfg.NodeID = GetNodeID()
	cfg.CheckinInterval = GetCheckinInterval()
	cfg.ServerCheckinInterval = GetServerCheckinInterval()
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
	cfg.GRPCSSL = "off"
	if IsGRPCSSL() {
		cfg.GRPCSSL = "on"
	}
	cfg.DisableRemoteIPCheck = "off"
	if DisableRemoteIPCheck() {
		cfg.DisableRemoteIPCheck = "on"
	}
	cfg.DisableDefaultNet = "off"
	if DisableDefaultNet() {
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

// GetVersion - version of netmaker
func GetVersion() string {
	version := "0.10.0"
	if config.Config.Server.Version != "" {
		version = config.Config.Server.Version
	}
	return version
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

// GetCheckinInterval - get check in interval for nodes
func GetCheckinInterval() string {
	seconds := "15"
	if os.Getenv("CHECKIN_INTERVAL") != "" {
		seconds = os.Getenv("CHECKIN_INTERVAL")
	} else if config.Config.Server.CheckinInterval != "" {
		seconds = config.Config.Server.CheckinInterval
	}
	return seconds
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

// GetGRPCConnString - get grpc conn string
func GetGRPCConnString() string {
	conn := ""
	if os.Getenv("SERVER_GRPC_CONN_STRING") != "" {
		conn = os.Getenv("SERVER_GRPC_CONN_STRING")
	} else if config.Config.Server.GRPCConnString != "" {
		conn = config.Config.Server.GRPCConnString
	}
	return conn
}

// GetCoreDNSAddr - gets the core dns address
func GetCoreDNSAddr() string {
	addr, _ := GetPublicIP()
	if os.Getenv("COREDNS_ADDR") != "" {
		addr = os.Getenv("COREDNS_ADDR")
	} else if config.Config.Server.CoreDNSAddr != "" {
		addr = config.Config.Server.GRPCConnString
	}
	return addr
}

// GetGRPCHost - get the grpc host url
func GetGRPCHost() string {
	serverhost := "127.0.0.1"
	remoteip, _ := GetPublicIP()
	if os.Getenv("SERVER_GRPC_HOST") != "" {
		serverhost = os.Getenv("SERVER_GRPC_HOST")
	} else if config.Config.Server.GRPCHost != "" {
		serverhost = config.Config.Server.GRPCHost
	} else if os.Getenv("SERVER_HOST") != "" {
		serverhost = os.Getenv("SERVER_HOST")
	} else {
		if remoteip != "" {
			serverhost = remoteip
		}
	}
	return serverhost
}

// GetGRPCPort - gets the grpc port
func GetGRPCPort() string {
	grpcport := "50051"
	if os.Getenv("GRPC_PORT") != "" {
		grpcport = os.Getenv("GRPC_PORT")
	} else if config.Config.Server.GRPCPort != "" {
		grpcport = config.Config.Server.GRPCPort
	}
	return grpcport
}

// GetMasterKey - gets the configured master key of server
func GetMasterKey() string {
	key := "secretkey"
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

// IsGRPCSSL - ssl grpc on or off
func IsGRPCSSL() bool {
	isssl := false
	if os.Getenv("GRPC_SSL") != "" {
		if os.Getenv("GRPC_SSL") == "on" {
			isssl = true
		}
	} else if config.Config.Server.DNSMode != "" {
		if config.Config.Server.DNSMode == "on" {
			isssl = true
		}
	}
	return isssl
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

// DisableDefaultNet - disable default net
func DisableDefaultNet() bool {
	disabled := false
	if os.Getenv("DISABLE_DEFAULT_NET") != "" {
		if os.Getenv("DISABLE_DEFAULT_NET") == "on" {
			disabled = true
		}
	} else if config.Config.Server.DisableDefaultNet != "" {
		if config.Config.Server.DisableDefaultNet == "on" {
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
	for _, ipserver := range iplist {
		resp, err := http.Get(ipserver)
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

// IsSplitDNS - checks if split dns is on
func IsSplitDNS() bool {
	issplit := false
	if os.Getenv("IS_SPLIT_DNS") == "yes" {
		issplit = true
	} else if config.Config.Server.SplitDNS == "yes" {
		issplit = true
	}
	return issplit
}

// IsSplitDNS - checks if split dns is on
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
	id = getMacAddr()
	if os.Getenv("NODE_ID") != "" {
		id = os.Getenv("NODE_ID")
	} else if config.Config.Server.NodeID != "" {
		id = config.Config.Server.NodeID
	}
	return id
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
func GetAuthProviderInfo() []string {
	var authProvider = ""
	if os.Getenv("AUTH_PROVIDER") != "" && os.Getenv("CLIENT_ID") != "" && os.Getenv("CLIENT_SECRET") != "" {
		authProvider = strings.ToLower(os.Getenv("AUTH_PROVIDER"))
		if authProvider == "google" || authProvider == "azure-ad" || authProvider == "github" {
			return []string{authProvider, os.Getenv("CLIENT_ID"), os.Getenv("CLIENT_SECRET")}
		} else {
			authProvider = ""
		}
	} else if config.Config.Server.AuthProvider != "" && config.Config.Server.ClientID != "" && config.Config.Server.ClientSecret != "" {
		authProvider = strings.ToLower(config.Config.Server.AuthProvider)
		if authProvider == "google" || authProvider == "azure-ad" || authProvider == "github" {
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

// GetMacAddr - get's mac address
func getMacAddr() string {
	ifas, err := net.Interfaces()
	if err != nil {
		return ""
	}
	var as []string
	for _, ifa := range ifas {
		a := ifa.HardwareAddr.String()
		if a != "" {
			as = append(as, a)
		}
	}
	return as[0]
}

// GetRce - sees if Rce is enabled, off by default
func GetRce() bool {
	return os.Getenv("RCE") == "on" || config.Config.Server.RCE == "on"
}
