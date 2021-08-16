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
}

// ServerConfig :
type ServerConfig struct {
	CoreDNSAddr          string `yaml:"corednsaddr"`
	APIConnString        string `yaml:"apiconn"`
	APIHost              string `yaml:"apihost"`
	APIPort              string `yaml:"apiport"`
	GRPCConnString       string `yaml:"grpcconn"`
	GRPCHost             string `yaml:"grpchost"`
	GRPCPort             string `yaml:"grpcport"`
	GRPCSecure           string `yaml:"grpcsecure"`
	MasterKey            string `yaml:"masterkey"`
	AllowedOrigin        string `yaml:"allowedorigin"`
	RestBackend          string `yaml:"restbackend"`
	AgentBackend         string `yaml:"agentbackend"`
	ClientMode           string `yaml:"clientmode"`
	DNSMode              string `yaml:"dnsmode"`
	DisableRemoteIPCheck string `yaml:"disableremoteipcheck"`
	DisableDefaultNet    string `yaml:"disabledefaultnet"`
	GRPCSSL              string `yaml:"grpcssl"`
	Version              string `yaml:"version"`
	DefaultNodeLimit     int32  `yaml:"defaultnodelimit"`
	Verbosity            int32  `yaml:"verbosity"`
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
