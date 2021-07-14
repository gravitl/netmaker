//Environment file for getting variables
//Currently the only thing it does is set the master password
//Should probably have it take over functions from OS such as port and mongodb connection details
//Reads from the config/environments/dev.yaml file by default
//TODO:  Add vars for mongo and remove from  OS vars in mongoconn
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
	Server    ServerConfig    `yaml:"server"`
	MongoConn MongoConnConfig `yaml:"mongoconn"`
	WG        WG              `yaml:"wg"`
}

// ServerConfig :
type ServerConfig struct {
	APIConnString        string `yaml:"apiconn"`
	APIHost              string `yaml:"apihost"`
	APIPort              string `yaml:"apiport"`
	GRPCConnString       string `yaml:"grpcconn"`
	GRPCHost             string `yaml:"grpchost"`
	GRPCPort             string `yaml:"grpcport"`
	GRPCSecure           string `yaml:"grpcsecure"`
	DefaultNodeLimit     int32  `yaml:"defaultnodelimit"`
	MasterKey            string `yaml:"masterkey"`
	AllowedOrigin        string `yaml:"allowedorigin"`
	RestBackend          string `yaml:"restbackend"`
	AgentBackend         string `yaml:"agentbackend"`
	ClientMode           string `yaml:"clientmode"`
	DNSMode              string `yaml:"dnsmode"`
	DisableRemoteIPCheck string `yaml:"disableremoteipcheck"`
	DisableDefaultNet    string `yaml:"disabledefaultnet"`
	GRPCSSL              string `yaml:"grpcssl"`
	Verbosity            int32  `yaml:"verbosity"`
}

type WG struct {
	RegisterKeyRequired string `yaml:"keyrequired"`
	GRPCWireGuard       string `yaml:"grpcwg"`
	GRPCWGInterface     string `yaml:"grpciface"`
	GRPCWGAddress       string `yaml:"grpcaddr"`
	GRPCWGAddressRange  string `yaml:"grpcaddrrange"`
	GRPCWGPort          string `yaml:"grpcport"`
	GRPCWGPubKey        string `yaml:"pubkey"`
	GRPCWGPrivKey       string `yaml:"privkey"`
}

type MongoConnConfig struct {
	User string `yaml:"user"`
	Pass string `yaml:"pass"`
	Host string `yaml:"host"`
	Port string `yaml:"port"`
	Opts string `yaml:"opts"`
}

//reading in the env file
func readConfig() *EnvironmentConfig {
	file := fmt.Sprintf("config/environments/%s.yaml", getEnv())
	f, err := os.Open(file)
	var cfg EnvironmentConfig
	if err != nil {
		//log.Fatal(err)
		//os.Exit(2)
		//log.Println("Unable to open config file at config/environments/" + getEnv())
		//log.Println("Will proceed with defaults or enironment variables (no config file).")
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
