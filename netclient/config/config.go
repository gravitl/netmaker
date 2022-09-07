package config

import (
	"crypto/ed25519"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/global_settings"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

var (
	configLock sync.Mutex
)

// ClientConfig - struct for dealing with client configuration
type ClientConfig struct {
	Server          models.ServerConfig `yaml:"server"`
	Node            models.Node         `yaml:"node"`
	NetworkSettings models.Network      `yaml:"networksettings"`
	Network         string              `yaml:"network"`
	Daemon          string              `yaml:"daemon"`
	OperatingSystem string              `yaml:"operatingsystem"`
	AccessKey       string              `yaml:"accesskey"`
	PublicIPService string              `yaml:"publicipservice"`
}

// RegisterRequest - struct for registation with netmaker server
type RegisterRequest struct {
	Key        ed25519.PrivateKey
	CommonName pkix.Name
}

// RegisterResponse - the response to register function
type RegisterResponse struct {
	CA         x509.Certificate
	CAPubKey   ed25519.PublicKey
	Cert       x509.Certificate
	CertPubKey ed25519.PublicKey
	Broker     string
	Port       string
}

// Write - writes the config of a client to disk
func Write(config *ClientConfig, network string) error {
	configLock.Lock()
	defer configLock.Unlock()
	if network == "" {
		err := errors.New("no network provided - exiting")
		return err
	}
	_, err := os.Stat(ncutils.GetNetclientPath() + "/config")
	if os.IsNotExist(err) {
		os.MkdirAll(ncutils.GetNetclientPath()+"/config", 0700)
	} else if err != nil {
		return err
	}
	home := ncutils.GetNetclientPathSpecific()

	file := fmt.Sprintf(home + "netconfig-" + network)
	f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	defer f.Close()

	err = yaml.NewEncoder(f).Encode(config)
	if err != nil {
		return err
	}
	return f.Sync()
}

// ConfigFileExists - return true if config file exists
func (config *ClientConfig) ConfigFileExists() bool {
	home := ncutils.GetNetclientPathSpecific()

	file := fmt.Sprintf(home + "netconfig-" + config.Network)
	info, err := os.Stat(file)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// ClientConfig.ReadConfig - used to read config from client disk into memory
func (config *ClientConfig) ReadConfig() {

	network := config.Network
	if network == "" {
		return
	}

	//home, err := homedir.Dir()
	home := ncutils.GetNetclientPathSpecific()

	file := fmt.Sprintf(home + "netconfig-" + network)
	//f, err := os.Open(file)
	f, err := os.OpenFile(file, os.O_RDONLY, 0600)
	if err != nil {
		logger.Log(1, "trouble opening file: ", err.Error())
		if err = ReplaceWithBackup(network); err != nil {
			log.Fatal(err)
		}
		f.Close()
		f, err = os.Open(file)
		if err != nil {
			log.Fatal(err)
		}

	}
	defer f.Close()
	if err := yaml.NewDecoder(f).Decode(&config); err != nil {
		logger.Log(0, "no config or invalid, replacing with backup")
		if err = ReplaceWithBackup(network); err != nil {
			log.Fatal(err)
		}
		f.Close()
		f, err = os.Open(file)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		if err := yaml.NewDecoder(f).Decode(&config); err != nil {
			log.Fatal(err)
		}
	}
}

// ModNodeConfig - overwrites the node inside client config on disk
func ModNodeConfig(node *models.Node) error {
	network := node.Network
	if network == "" {
		return errors.New("no network provided")
	}
	var modconfig ClientConfig
	if FileExists(ncutils.GetNetclientPathSpecific() + "netconfig-" + network) {
		useconfig, err := ReadConfig(network)
		if err != nil {
			return err
		}
		modconfig = *useconfig
	}

	modconfig.Node = (*node)
	modconfig.NetworkSettings = node.NetworkSettings
	return Write(&modconfig, network)
}

// ModNodeConfig - overwrites the server settings inside client config on disk
func ModServerConfig(scfg *models.ServerConfig, network string) error {
	var modconfig ClientConfig
	if FileExists(ncutils.GetNetclientPathSpecific() + "netconfig-" + network) {
		useconfig, err := ReadConfig(network)
		if err != nil {
			return err
		}
		modconfig = *useconfig
	}

	modconfig.Server = (*scfg)
	return Write(&modconfig, network)
}

// SaveBackup - saves a backup file of a given network
func SaveBackup(network string) error {

	var configPath = ncutils.GetNetclientPathSpecific() + "netconfig-" + network
	var backupPath = ncutils.GetNetclientPathSpecific() + "backup.netconfig-" + network
	if FileExists(configPath) {
		input, err := os.ReadFile(configPath)
		if err != nil {
			logger.Log(0, "failed to read ", configPath, " to make a backup")
			return err
		}
		if err = os.WriteFile(backupPath, input, 0600); err != nil {
			logger.Log(0, "failed to copy backup to ", backupPath)
			return err
		}
	}
	return nil
}

// ReplaceWithBackup - replaces netconfig file with backup
func ReplaceWithBackup(network string) error {
	var backupPath = ncutils.GetNetclientPathSpecific() + "backup.netconfig-" + network
	var configPath = ncutils.GetNetclientPathSpecific() + "netconfig-" + network
	if FileExists(backupPath) {
		input, err := os.ReadFile(backupPath)
		if err != nil {
			logger.Log(0, "failed to read file ", backupPath, " to backup network: ", network)
			return err
		}
		if err = os.WriteFile(configPath, input, 0600); err != nil {
			logger.Log(0, "failed backup ", backupPath, " to ", configPath)
			return err
		}
	}
	logger.Log(0, "used backup file for network: ", network)
	return nil
}

// GetCLIConfig - gets the cli flags as a config
func GetCLIConfig(c *cli.Context) (ClientConfig, string, error) {
	var cfg ClientConfig
	if c.String("token") != "" {
		accesstoken, err := ParseAccessToken(c.String("token"))
		if err != nil {
			return cfg, "", err
		}
		cfg.Network = accesstoken.ClientConfig.Network
		cfg.Node.Network = accesstoken.ClientConfig.Network
		cfg.AccessKey = accesstoken.ClientConfig.Key
		cfg.Node.LocalRange = accesstoken.ClientConfig.LocalRange
		//cfg.Server.Server = accesstoken.ServerConfig.Server
		cfg.Server.API = accesstoken.APIConnString
		if c.String("key") != "" {
			cfg.AccessKey = c.String("key")
		}
		if c.String("network") != "all" {
			cfg.Network = c.String("network")
			cfg.Node.Network = c.String("network")
		}
		if c.String("localrange") != "" {
			cfg.Node.LocalRange = c.String("localrange")
		}
		if c.String("corednsaddr") != "" {
			cfg.Server.CoreDNSAddr = c.String("corednsaddr")
		}
		if c.String("apiserver") != "" {
			cfg.Server.API = c.String("apiserver")
		}
	} else {
		cfg.AccessKey = c.String("key")
		cfg.Network = c.String("network")
		cfg.Node.Network = c.String("network")
		cfg.Node.LocalRange = c.String("localrange")
		cfg.Server.CoreDNSAddr = c.String("corednsaddr")
		cfg.Server.API = c.String("apiserver")
	}
	cfg.PublicIPService = c.String("publicipservice")
	// populate the map as we're not running as a daemon so won't be building the map otherwise
	// (and the map will be used by GetPublicIP()).
	global_settings.PublicIPServices[cfg.Network] = cfg.PublicIPService
	cfg.Node.Name = c.String("name")
	cfg.Node.Interface = c.String("interface")
	cfg.Node.Password = c.String("password")
	cfg.Node.MacAddress = c.String("macaddress")
	cfg.Node.LocalAddress = c.String("localaddress")
	cfg.Node.Address = c.String("address")
	cfg.Node.Address6 = c.String("address6")
	//cfg.Node.Roaming = c.String("roaming")
	cfg.Node.DNSOn = c.String("dnson")
	cfg.Node.IsLocal = c.String("islocal")
	cfg.Node.IsStatic = c.String("isstatic")
	cfg.Node.PostUp = c.String("postup")
	cfg.Node.PostDown = c.String("postdown")
	cfg.Node.ListenPort = int32(c.Int("port"))
	cfg.Node.PersistentKeepalive = int32(c.Int("keepalive"))
	cfg.Node.PublicKey = c.String("publickey")
	privateKey := c.String("privatekey")
	cfg.Node.Endpoint = c.String("endpoint")
	cfg.Node.IPForwarding = c.String("ipforwarding")
	cfg.OperatingSystem = c.String("operatingsystem")
	cfg.Daemon = c.String("daemon")
	cfg.Node.UDPHolePunch = c.String("udpholepunch")
	cfg.Node.MTU = int32(c.Int("mtu"))

	return cfg, privateKey, nil
}

// ReadConfig - reads a config of a client from disk for specified network
func ReadConfig(network string) (*ClientConfig, error) {
	if network == "" {
		err := errors.New("no network provided - exiting")
		return nil, err
	}
	home := ncutils.GetNetclientPathSpecific()
	file := fmt.Sprintf(home + "netconfig-" + network)
	f, err := os.Open(file)
	if err != nil {
		if err = ReplaceWithBackup(network); err != nil {
			return nil, err
		}
		f, err = os.Open(file)
		if err != nil {
			return nil, err
		}
	}
	defer f.Close()

	var cfg ClientConfig
	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		if err = ReplaceWithBackup(network); err != nil {
			return nil, err
		}
		f.Close()
		f, err = os.Open(file)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
			return nil, err
		}
	}

	return &cfg, err
}

// FileExists - checks if a file exists on disk
func FileExists(f string) bool {
	info, err := os.Stat(f)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// GetNode - parses a network specified client config for node data
func GetNode(network string) models.Node {

	modcfg, err := ReadConfig(network)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	var node models.Node
	node.Fill(&modcfg.Node)

	return node
}
