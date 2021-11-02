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

//setting dev by default
func getEnv() string {

	env := os.Getenv("NETMAKER_ENV")

	if len(env) == 0 {
		return "dev"
	}

	return env
}

// Config : application config stored as global variable
var Config *EnvironmentConfig

// EnvironmentConfig :
type EnvironmentConfig struct {
	Server ServerConfig `yaml:"server"`
	SQL    SQLConfig    `yaml:"sql"`
}

// ServerConfig :
type ServerConfig struct {
	CoreDNSAddr           string `yaml:"corednsaddr"`
	APIConnString         string `yaml:"apiconn"`
	APIHost               string `yaml:"apihost"`
	APIPort               string `yaml:"apiport"`
	GRPCConnString        string `yaml:"grpcconn"`
	GRPCHost              string `yaml:"grpchost"`
	GRPCPort              string `yaml:"grpcport"`
	GRPCSecure            string `yaml:"grpcsecure"`
	MasterKey             string `yaml:"masterkey"`
	AllowedOrigin         string `yaml:"allowedorigin"`
	NodeID                string `yaml:"nodeid"`
	RestBackend           string `yaml:"restbackend"`
	AgentBackend          string `yaml:"agentbackend"`
	ClientMode            string `yaml:"clientmode"`
	DNSMode               string `yaml:"dnsmode"`
	SplitDNS              string `yaml:"splitdns"`
	DisableRemoteIPCheck  string `yaml:"disableremoteipcheck"`
	DisableDefaultNet     string `yaml:"disabledefaultnet"`
	GRPCSSL               string `yaml:"grpcssl"`
	Version               string `yaml:"version"`
	SQLConn               string `yaml:"sqlconn"`
	Platform              string `yaml:"platform"`
	Database              string `yaml:"database"`
	CheckinInterval       string `yaml:"checkininterval"`
	DefaultNodeLimit      int32  `yaml:"defaultnodelimit"`
	Verbosity             int32  `yaml:"verbosity"`
	ServerCheckinInterval int64  `yaml:"servercheckininterval"`
	AuthProvider          string `yaml:"authprovider"`
	ClientID              string `yaml:"clientid"`
	ClientSecret          string `yaml:"clientsecret"`
	FrontendURL           string `yaml:"frontendurl"`
	EtcdAddresses         string `yaml:"etcdaddresses"`
	EtcdCertPath          string `yaml:"etcdcertpath"`
	EtcdCACertPath        string `yaml:"etcdcacertpath"`
	EtcdKeyPath           string `yaml:"etcdkeypath"`
	EtcdSSL               string `yaml:"etcdssl"`
}

// Generic SQL Config
type SQLConfig struct {
	Host     string `yaml:"host"`
	Port     int32  `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	DB       string `yaml:"db"`
	SSLMode  string `yaml:"sslmode"`
}

//reading in the env file
func readConfig() *EnvironmentConfig {
	file := fmt.Sprintf("config/environments/%s.yaml", getEnv())
	f, err := os.Open(file)
	var cfg EnvironmentConfig
	if err != nil {
		return &cfg
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		log.Fatal(err)
		os.Exit(2)
	}
	return &cfg

}

func init() {
	Config = readConfig()
}
