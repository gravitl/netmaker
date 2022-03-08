//Environment file for getting variables
//Currently the only thing it does is set the master password
//Should probably have it take over functions from OS such as port and mongodb connection details
//Reads from the config/environments/dev.yaml file by default
package config

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// setting dev by default
func getEnv() string {

	env := os.Getenv("NETMAKER_ENV")

	if len(env) == 0 {
		return "dev"
	}

	return env
}

// Config : application config stored as global variable
var Config *EnvironmentConfig
var SetupErr error

// EnvironmentConfig - environment conf struct
type EnvironmentConfig struct {
	Server ServerConfig `yaml:"server"`
	SQL    SQLConfig    `yaml:"sql"`
}

// ServerConfig - server conf struct
type ServerConfig struct {
	CoreDNSAddr           string `yaml:"corednsaddr"`
	APIConnString         string `yaml:"apiconn"`
	APIHost               string `yaml:"apihost"`
	APIPort               string `yaml:"apiport"`
	GRPCConnString        string `yaml:"grpcconn"`
	GRPCHost              string `yaml:"grpchost"`
	GRPCPort              string `yaml:"grpcport"`
	GRPCSecure            string `yaml:"grpcsecure"`
	MQHOST                string `yaml:"mqhost"`
	MasterKey             string `yaml:"masterkey"`
	DNSKey                string `yaml:"dnskey"`
	AllowedOrigin         string `yaml:"allowedorigin"`
	NodeID                string `yaml:"nodeid"`
	RestBackend           string `yaml:"restbackend"`
	AgentBackend          string `yaml:"agentbackend"`
	MessageQueueBackend   string `yaml:"messagequeuebackend"`
	ClientMode            string `yaml:"clientmode"`
	DNSMode               string `yaml:"dnsmode"`
	DisableRemoteIPCheck  string `yaml:"disableremoteipcheck"`
	GRPCSSL               string `yaml:"grpcssl"`
	Version               string `yaml:"version"`
	SQLConn               string `yaml:"sqlconn"`
	Platform              string `yaml:"platform"`
	Database              string `yaml:"database"`
	DefaultNodeLimit      int32  `yaml:"defaultnodelimit"`
	Verbosity             int32  `yaml:"verbosity"`
	ServerCheckinInterval int64  `yaml:"servercheckininterval"`
	AuthProvider          string `yaml:"authprovider"`
	ClientID              string `yaml:"clientid"`
	ClientSecret          string `yaml:"clientsecret"`
	FrontendURL           string `yaml:"frontendurl"`
	DisplayKeys           string `yaml:"displaykeys"`
	AzureTenant           string `yaml:"azuretenant"`
	RCE                   string `yaml:"rce"`
	Debug                 bool   `yaml:"debug"`
	Telemetry             string `yaml:"telemetry"`
	ManageIPTables        string `yaml:"manageiptables"`
	PortForwardServices   string `yaml:"portforwardservices"`
	HostNetwork           string `yaml:"hostnetwork"`
	CommsCIDR             string `yaml:"commscidr"`
	MQPort                string `yaml:"mqport"`
	CommsID               string `yaml:"commsid"`
}

// SQLConfig - Generic SQL Config
type SQLConfig struct {
	Host     string `yaml:"host"`
	Port     int32  `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	DB       string `yaml:"db"`
	SSLMode  string `yaml:"sslmode"`
}

// reading in the env file
func readConfig() (*EnvironmentConfig, error) {
	file := fmt.Sprintf("environments/%s.yaml", getEnv())
	f, err := os.Open(file)
	var cfg EnvironmentConfig
	if err != nil {
		return &cfg, err
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	if decoder.Decode(&cfg) != nil {
		return &cfg, err
	}
	return &cfg, err

}

func init() {
	if Config, SetupErr = readConfig(); SetupErr != nil {
		log.Fatal(SetupErr)
		os.Exit(2)
	}
}
