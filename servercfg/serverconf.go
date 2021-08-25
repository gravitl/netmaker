package servercfg

import (
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

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
	if IsRestBackend() {
		cfg.RestBackend = "on"
	}
	cfg.AgentBackend = "off"
	if IsAgentBackend() {
		cfg.AgentBackend = "on"
	}
	cfg.ClientMode = "off"
	if IsClientMode() {
		cfg.ClientMode = "on"
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
	version := "0.7.3"
	if config.Config.Server.Version != "" {
		version = config.Config.Server.Version
	}
	return version
}
func GetDB() string {
	database := "rqlite"
	if os.Getenv("DATABASE") == "sqlite" {
		database = os.Getenv("DATABASE")
	} else if config.Config.Server.Database == "sqlite" {
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
func IsClientMode() bool {
	isclient := true
	if os.Getenv("CLIENT_MODE") != "" {
		if os.Getenv("CLIENT_MODE") == "off" {
			isclient = false
		}
	} else if config.Config.Server.ClientMode != "" {
		if config.Config.Server.ClientMode == "off" {
			isclient = false
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
