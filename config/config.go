//Environment file for getting variables
//Currently the only thing it does is set the master password
//Should probably have it take over functions from OS such as port and mongodb connection details
//Reads from the config/environments/dev.yaml file by default
//TODO:  Add vars for mongo and remove from  OS vars in mongoconn
package config

import (
  "os"
  "fmt"
  "log"
  "gopkg.in/yaml.v3"
)

//setting dev by default
func getEnv() string {

  env := os.Getenv("APP_ENV")

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
  MongoConn MongoConnConfig `yaml:"mongoconn"`
}

// ServerConfig :
type ServerConfig struct {
  Host   string  `yaml:"host"`
  ApiPort   string `yaml:"apiport"`
  GrpcPort   string `yaml:"grpcport"`
  MasterKey	string `yaml:"masterkey"`
  AllowedOrigin	string `yaml:"allowedorigin"`
  RestBackend bool `yaml:"restbackend"`
  AgentBackend bool `yaml:"agentbackend"`
}

type MongoConnConfig struct {
  User   string  `yaml:"user"`
  Pass   string  `yaml:"pass"`
  Host   string  `yaml:"host"`
  Port   string  `yaml:"port"`
  Opts   string  `yaml:"opts"`
}


//reading in the env file
func readConfig() *EnvironmentConfig {
  file := fmt.Sprintf("config/environments/%s.yaml", getEnv())
  f, err := os.Open(file)
  if err != nil {
    log.Fatal(err)
    os.Exit(2)
  }
  defer f.Close()

  var cfg EnvironmentConfig
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
