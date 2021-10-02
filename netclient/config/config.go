package config

import (
	//"github.com/davecgh/go-spew/spew"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

type GlobalConfig struct {
	GRPCWireGuard string `yaml:"grpcwg"`
	Client        models.IntClient
}

type ClientConfig struct {
	Server          ServerConfig `yaml:"server"`
	Node            models.Node  `yaml:"node"`
	Network         string       `yaml:"network"`
	Daemon          string       `yaml:"daemon"`
	OperatingSystem string       `yaml:"operatingsystem"`
	DebugJoin       bool         `yaml:"debugjoin"`
}
type ServerConfig struct {
	CoreDNSAddr   string `yaml:"corednsaddr"`
	GRPCAddress   string `yaml:"grpcaddress"`
	APIAddress    string `yaml:"apiaddress"`
	AccessKey     string `yaml:"accesskey"`
	GRPCSSL       string `yaml:"grpcssl"`
	GRPCWireGuard string `yaml:"grpcwg"`
	CheckinInterval string `yaml:"checkininterval"`
}

//reading in the env file
func Write(config *ClientConfig, network string) error {
	if network == "" {
		err := errors.New("no network provided - exiting")
		return err
	}
	_, err := os.Stat(ncutils.GetNetclientPath()+"/config")
	if os.IsNotExist(err) {
		os.MkdirAll(ncutils.GetNetclientPath()+"/config", 0744)
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
	return err
}

func WriteServer(server string, accesskey string, network string) error {
	if network == "" {
		err := errors.New("no network provided - exiting")
		return err
	}
	nofile := false
	//home, err := homedir.Dir()
	_, err := os.Stat(ncutils.GetNetclientPath()+"/config")
	if os.IsNotExist(err) {
		os.MkdirAll(ncutils.GetNetclientPath()+"/config", 0744)
	} else if err != nil {
		fmt.Println("couldnt find or create", ncutils.GetNetclientPath())
		return err
	}
	home := ncutils.GetNetclientPathSpecific()

	file := fmt.Sprintf(home + "netconfig-" + network)
	//f, err := os.Open(file)
	f, err := os.OpenFile(file, os.O_CREATE|os.O_RDWR, 0666)
	//f, err := ioutil.ReadFile(file)
	if err != nil {
		fmt.Println("couldnt open netconfig-" + network)
		fmt.Println(err)
		nofile = true
		//err = nil
		return err
	}
	defer f.Close()

	//cfg := &ClientConfig{}
	var cfg ClientConfig

	if !nofile {
		fmt.Println("Writing to existing config file at " + home + "netconfig-" + network)
		decoder := yaml.NewDecoder(f)
		err = decoder.Decode(&cfg)
		//err = yaml.Unmarshal(f, &cfg)
		if err != nil {
			//fmt.Println(err)
			//return err
		}
		f.Close()
		f, err = os.OpenFile(file, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
		if err != nil {
			fmt.Println("couldnt open netconfig")
			fmt.Println(err)
			nofile = true
			//err = nil
			return err
		}
		defer f.Close()

		if err != nil {
			fmt.Println("trouble opening file")
			fmt.Println(err)
		}

		cfg.Server.GRPCAddress = server
		cfg.Server.AccessKey = accesskey

		err = yaml.NewEncoder(f).Encode(cfg)
		//_, err = yaml.Marshal(f, &cfg)
		if err != nil {
			fmt.Println("trouble encoding file")
			return err
		}
	} else {
		fmt.Println("Creating new config file at " + home + "netconfig-" + network)

		cfg.Server.GRPCAddress = server
		cfg.Server.AccessKey = accesskey

		newf, err := os.Create(home + "netconfig-" + network)
		err = yaml.NewEncoder(newf).Encode(cfg)
		defer newf.Close()
		if err != nil {
			return err
		}
	}

	return err
}

func (config *ClientConfig) ReadConfig() {

	nofile := false
	//home, err := homedir.Dir()
	home := ncutils.GetNetclientPathSpecific()
	file := fmt.Sprintf(home + "netconfig-" + config.Network)
	//f, err := os.Open(file)
	f, err := os.OpenFile(file, os.O_RDONLY, 0666)
	if err != nil {
		fmt.Println("trouble opening file")
		fmt.Println(err)
		nofile = true
		//fmt.Println("Could not access " + home + "/.netconfig,  proceeding...")
	}
	defer f.Close()

	//var cfg ClientConfig

	if !nofile {
		decoder := yaml.NewDecoder(f)
		err = decoder.Decode(&config)
		if err != nil {
			fmt.Println("no config or invalid")
			fmt.Println(err)
			log.Fatal(err)
		} else {
			config.Node.SetID()
			//config = cfg
		}
	}
}

func ModConfig(node *models.Node) error {
	network := node.Network
	if network == "" {
		return errors.New("no network provided")
	}
	var modconfig ClientConfig
	var err error
	if FileExists(ncutils.GetNetclientPathSpecific() + "netconfig-" + network) {
		useconfig, err := ReadConfig(network)
		if err != nil {
			return err
		}
		modconfig = *useconfig
	}

	modconfig.Node = (*node)
	err = Write(&modconfig, network)
	return err
}

func GetCLIConfig(c *cli.Context) (ClientConfig, string, error) {
	var cfg ClientConfig
	if c.String("token") != "" {
		tokenbytes, err := base64.StdEncoding.DecodeString(c.String("token"))
		if err != nil {
			log.Println("error decoding token")
			return cfg, "", err
		}
		var accesstoken models.AccessToken
		if err := json.Unmarshal(tokenbytes, &accesstoken); err != nil {
			log.Println("error converting token json to object", tokenbytes)
			return cfg, "", err
		}

		if accesstoken.ServerConfig.APIConnString != "" {
			cfg.Server.APIAddress = accesstoken.ServerConfig.APIConnString
		} else {
			cfg.Server.APIAddress = accesstoken.ServerConfig.APIHost
			if accesstoken.ServerConfig.APIPort != "" {
				cfg.Server.APIAddress = cfg.Server.APIAddress + ":" + accesstoken.ServerConfig.APIPort
			}
		}
		if accesstoken.ServerConfig.GRPCConnString != "" {
			cfg.Server.GRPCAddress = accesstoken.ServerConfig.GRPCConnString
		} else {
			cfg.Server.GRPCAddress = accesstoken.ServerConfig.GRPCHost
			if accesstoken.ServerConfig.GRPCPort != "" {
				cfg.Server.GRPCAddress = cfg.Server.GRPCAddress + ":" + accesstoken.ServerConfig.GRPCPort
			}
		}

		cfg.Network = accesstoken.ClientConfig.Network
		cfg.Node.Network = accesstoken.ClientConfig.Network
		cfg.Server.AccessKey = accesstoken.ClientConfig.Key
		cfg.Node.LocalRange = accesstoken.ClientConfig.LocalRange
		cfg.Server.GRPCSSL = accesstoken.ServerConfig.GRPCSSL
		cfg.Server.CheckinInterval = accesstoken.ServerConfig.CheckinInterval
		cfg.Server.GRPCWireGuard = accesstoken.WG.GRPCWireGuard
		cfg.Server.CoreDNSAddr = accesstoken.ServerConfig.CoreDNSAddr
		if c.String("grpcserver") != "" {
			cfg.Server.GRPCAddress = c.String("grpcserver")
		}
		if c.String("apiserver") != "" {
			cfg.Server.APIAddress = c.String("apiserver")
		}
		if c.String("key") != "" {
			cfg.Server.AccessKey = c.String("key")
		}
		if c.String("network") != "all" {
			cfg.Network = c.String("network")
			cfg.Node.Network = c.String("network")
		}
		if c.String("localrange") != "" {
			cfg.Node.LocalRange = c.String("localrange")
		}
		if c.String("grpcssl") != "" {
			cfg.Server.GRPCSSL = c.String("grpcssl")
		}
		if c.String("corednsaddr") != "" {
			cfg.Server.CoreDNSAddr = c.String("corednsaddr")
		}
		if c.String("grpcwg") != "" {
			cfg.Server.GRPCWireGuard = c.String("grpcwg")
		}
		if c.String("checkininterval") != "" {
			cfg.Server.CheckinInterval = c.String("checkininterval")
		}

	} else {
		cfg.Server.GRPCAddress = c.String("grpcserver")
		cfg.Server.APIAddress = c.String("apiserver")
		cfg.Server.AccessKey = c.String("key")
		cfg.Network = c.String("network")
		cfg.Node.Network = c.String("network")
		cfg.Node.LocalRange = c.String("localrange")
		cfg.Server.GRPCWireGuard = c.String("grpcwg")
		cfg.Server.GRPCSSL = c.String("grpcssl")
		cfg.Server.CoreDNSAddr = c.String("corednsaddr")
		cfg.Server.CheckinInterval = c.String("checkininterval")
	}
	cfg.Node.Name = c.String("name")
	cfg.Node.Interface = c.String("interface")
	cfg.Node.Password = c.String("password")
	cfg.Node.MacAddress = c.String("macaddress")
	cfg.Node.LocalAddress = c.String("localaddress")
	cfg.Node.Address = c.String("address")
	cfg.Node.Address6 = c.String("addressIPV6")
	cfg.Node.Roaming = c.String("roaming")
	cfg.Node.DNSOn = c.String("dnson")
	cfg.Node.IsLocal = c.String("islocal")
	cfg.Node.IsDualStack = c.String("isdualstack")
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

	if cfg.Server.CheckinInterval == "" {
		cfg.Server.CheckinInterval = "15"
	}

	return cfg, privateKey, nil
}

func ReadConfig(network string) (*ClientConfig, error) {
	if network == "" {
		err := errors.New("no network provided - exiting")
		return nil, err
	}
	nofile := false
	home := ncutils.GetNetclientPathSpecific()
	file := fmt.Sprintf(home + "netconfig-" + network)
	f, err := os.Open(file)

	if err != nil {
		nofile = true
	}
	defer f.Close()

	var cfg ClientConfig

	if !nofile {
		decoder := yaml.NewDecoder(f)
		err = decoder.Decode(&cfg)
		if err != nil {
			fmt.Println("trouble decoding file")
			return nil, err
		}
	}
	return &cfg, err
}

func FileExists(f string) bool {
	info, err := os.Stat(f)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func GetNode(network string) models.Node {

	modcfg, err := ReadConfig(network)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	var node models.Node
	node.Fill(&modcfg.Node)

	return node
}
