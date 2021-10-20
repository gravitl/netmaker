package servercfg

import (
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gravitl/netmaker/config"
)

func SetHost() error {
	remoteip, err := GetPublicIP()
	if err != nil {
		return err
	}
	os.Setenv("SERVER_HOST", remoteip)
	return nil
}
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
	cfg.AllowedOrigin = GetAllowedOrigin()
	cfg.RestBackend = "off"
	cfg.Verbosity = GetVerbose()
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
	return cfg
}
func GetAPIConnString() string {
	conn := ""
	if os.Getenv("SERVER_API_CONN_STRING") != "" {
		conn = os.Getenv("SERVER_API_CONN_STRING")
	} else if config.Config.Server.APIConnString != "" {
		conn = config.Config.Server.APIConnString
	}
	return conn
}
func GetVersion() string {
	version := "0.8.4"
	if config.Config.Server.Version != "" {
		version = config.Config.Server.Version
	}
	return version
}
func GetDB() string {
	database := "sqlite"
	if os.Getenv("DATABASE") != "" {
		database = os.Getenv("DATABASE")
	} else if config.Config.Server.Database != "" {
		database = config.Config.Server.Database
	}
	return database
}
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
func GetPodIP() string {
	podip := "127.0.0.1"
	if os.Getenv("POD_IP") != "" {
		podip = os.Getenv("POD_IP")
	}
	return podip
}

func GetAPIPort() string {
	apiport := "8081"
	if os.Getenv("API_PORT") != "" {
		apiport = os.Getenv("API_PORT")
	} else if config.Config.Server.APIPort != "" {
		apiport = config.Config.Server.APIPort
	}
	return apiport
}

func GetCheckinInterval() string {
	seconds := "15"
	if os.Getenv("CHECKIN_INTERVAL") != "" {
		seconds = os.Getenv("CHECKIN_INTERVAL")
	} else if config.Config.Server.CheckinInterval != "" {
		seconds = config.Config.Server.CheckinInterval
	}
	return seconds
}

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
func GetGRPCConnString() string {
	conn := ""
	if os.Getenv("SERVER_GRPC_CONN_STRING") != "" {
		conn = os.Getenv("SERVER_GRPC_CONN_STRING")
	} else if config.Config.Server.GRPCConnString != "" {
		conn = config.Config.Server.GRPCConnString
	}
	return conn
}

func GetCoreDNSAddr() string {
	addr, _ := GetPublicIP()
	if os.Getenv("COREDNS_ADDR") != "" {
		addr = os.Getenv("COREDNS_ADDR")
	} else if config.Config.Server.CoreDNSAddr != "" {
		addr = config.Config.Server.GRPCConnString
	}
	return addr
}

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
func GetGRPCPort() string {
	grpcport := "50051"
	if os.Getenv("GRPC_PORT") != "" {
		grpcport = os.Getenv("GRPC_PORT")
	} else if config.Config.Server.GRPCPort != "" {
		grpcport = config.Config.Server.GRPCPort
	}
	return grpcport
}
func GetMasterKey() string {
	key := "secretkey"
	if os.Getenv("MASTER_KEY") != "" {
		key = os.Getenv("MASTER_KEY")
	} else if config.Config.Server.MasterKey != "" {
		key = config.Config.Server.MasterKey
	}
	return key
}
func GetAllowedOrigin() string {
	allowedorigin := "*"
	if os.Getenv("CORS_ALLOWED_ORIGIN") != "" {
		allowedorigin = os.Getenv("CORS_ALLOWED_ORIGIN")
	} else if config.Config.Server.AllowedOrigin != "" {
		allowedorigin = config.Config.Server.AllowedOrigin
	}
	return allowedorigin
}
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
func IsClientMode() string {
	isclient := "on"
	if os.Getenv("CLIENT_MODE") != "" {
		if os.Getenv("CLIENT_MODE") == "off" {
			isclient = "off"
		}
		if os.Getenv("CLIENT_MODE") == "contained" {
			isclient = "contained"
		}
	} else if config.Config.Server.ClientMode != "" {
		if config.Config.Server.ClientMode == "off" {
			isclient = "off"
		}
		if config.Config.Server.ClientMode == "contained" {
			isclient = "contained"
		}
	}
	return isclient
}
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
func GetPublicIP() (string, error) {

	endpoint := ""
	var err error

	iplist := []string{"http://ip.server.gravitl.com", "https://ifconfig.me", "http://api.ipify.org", "http://ipinfo.io/ip"}
	for _, ipserver := range iplist {
		resp, err := http.Get(ipserver)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			bodyBytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				continue
			}
			endpoint = string(bodyBytes)
			break
		}
	}
	if err == nil && endpoint == "" {
		err = errors.New("Public Address Not Found.")
	}
	return endpoint, err
}
func GetVerbose() int32 {
	level, err := strconv.Atoi(os.Getenv("VERBOSITY"))
	if err != nil || level < 0 {
		level = 0
	}
	if level > 3 {
		level = 3
	}
	return int32(level)
}

func GetPlatform() string {
	platform := "linux"
	if os.Getenv("PLATFORM") != "" {
		platform = os.Getenv("PLATFORM")
	} else if config.Config.Server.Platform != "" {
		platform = config.Config.Server.SQLConn
	}
	return platform
}

func GetSQLConn() string {
	sqlconn := "http://"
	if os.Getenv("SQL_CONN") != "" {
		sqlconn = os.Getenv("SQL_CONN")
	} else if config.Config.Server.SQLConn != "" {
		sqlconn = config.Config.Server.SQLConn
	}
	return sqlconn
}

func IsSplitDNS() bool {
	issplit := false
	if os.Getenv("IS_SPLIT_DNS") == "yes" {
		issplit = true
	} else if config.Config.Server.SplitDNS == "yes" {
		issplit = true
	}
	return issplit
}

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
