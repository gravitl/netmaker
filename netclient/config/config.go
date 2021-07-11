package config

import (
	//"github.com/davecgh/go-spew/spew"
	"github.com/urfave/cli/v2"
	"os"
        "encoding/base64"
	"errors"
	"fmt"
	"log"
        "encoding/json"
	"gopkg.in/yaml.v3"
	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/models"
)
type GlobalConfig struct {
	Client models.IntClient
}

type ClientConfig struct {
	Server ServerConfig `yaml:"server"`
	Node NodeConfig `yaml:"node"`
	Network string `yaml:"network"`
	Daemon string `yaml:"daemon"`
	OperatingSystem string `yaml:"operatingsystem"`
}
type ServerConfig struct {
        GRPCAddress string `yaml:"grpcaddress"`
        APIAddress string `yaml:"apiaddress"`
        AccessKey string `yaml:"accesskey"`
        GRPCSSL string `yaml:"grpcssl"`
        GRPCWireGuard string `yaml:"grpcwg"`
}

type ListConfig struct {
        Name string `yaml:"name"`
        Interface string `yaml:"interface"`
        PrivateIPv4 string `yaml:"wgaddress"`
        PrivateIPv6 string `yaml:"wgaddress6"`
        PublicEndpoint string `yaml:"endpoint"`
}

type NodeConfig struct {
        Name string `yaml:"name"`
        Interface string `yaml:"interface"`
        Network string `yaml:"network"`
        Password string `yaml:"password"`
        MacAddress string `yaml:"macaddress"`
        LocalAddress string `yaml:"localaddress"`
        WGAddress string `yaml:"wgaddress"`
        WGAddress6 string `yaml:"wgaddress6"`
        Roaming string `yaml:"roaming"`
        DNS string `yaml:"dns"`
        IsLocal string `yaml:"islocal"`
        IsDualStack string `yaml:"isdualstack"`
        IsIngressGateway string `yaml:"isingressgateway"`
        AllowedIPs []string `yaml:"allowedips"`
        LocalRange string `yaml:"localrange"`
        PostUp string `yaml:"postup"`
        PostDown string `yaml:"postdown"`
        Port int32 `yaml:"port"`
        KeepAlive int32 `yaml:"keepalive"`
        PublicKey string `yaml:"publickey"`
        ServerPubKey string `yaml:"serverpubkey"`
        PrivateKey string `yaml:"privatekey"`
        Endpoint string `yaml:"endpoint"`
        PostChanges string `yaml:"postchanges"`
        StaticIP string `yaml:"staticip"`
        StaticPubKey string `yaml:"staticpubkey"`
        IPForwarding string `yaml:"ipforwarding"`
}

//reading in the env file
func Write(config *ClientConfig, network string) error{
	if network == "" {
		err := errors.New("No network provided. Exiting.")
		return err
	}
        _, err := os.Stat("/etc/netclient") 
	if os.IsNotExist(err) {
		      os.Mkdir("/etc/netclient", 744)
	} else if err != nil {
                return err
        }
	home := "/etc/netclient"

        if err != nil {
                log.Fatal(err)
        }
        file := fmt.Sprintf(home + "/netconfig-" + network)
        f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
        defer f.Close()

	err = yaml.NewEncoder(f).Encode(config)
	if err != nil {
		return err
	}
        return err
}
//reading in the env file
func WriteGlobal(config *GlobalConfig) error{
        _, err := os.Stat("/etc/netclient") 
        if os.IsNotExist(err) {
                      os.Mkdir("/etc/netclient", 744)
        } else if err != nil {
                return err
        }
        home := "/etc/netclient"

        if err != nil {
                log.Fatal(err)
        }
        file := fmt.Sprintf(home + "/netconfig-global-001")
        f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
        defer f.Close()

        err = yaml.NewEncoder(f).Encode(config)
        if err != nil {
                return err
        }
        return err
}
func WriteServer(server string, accesskey string, network string) error{
        if network == "" {
                err := errors.New("No network provided. Exiting.")
                return err
        }
        nofile := false
        //home, err := homedir.Dir()
        _, err := os.Stat("/etc/netclient")
	if os.IsNotExist(err) {
                os.Mkdir("/etc/netclient", 744)
        } else if err != nil {
		fmt.Println("couldnt find or create /etc/netclient")
                return err
        }
        home := "/etc/netclient"

	file := fmt.Sprintf(home + "/netconfig-" + network)
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
		fmt.Println("Writing to existing config file at " + home + "/netconfig-" + network)
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
                fmt.Println("Creating new config file at " + home + "/netconfig-" + network)

                cfg.Server.GRPCAddress = server
                cfg.Server.AccessKey = accesskey

                newf, err := os.Create(home + "/netconfig-" + network)
                err = yaml.NewEncoder(newf).Encode(cfg)
                defer newf.Close()
                if err != nil {
                        return err
                }
        }

        return err
}



func(config *ClientConfig) ReadConfig() {

	nofile := false
	//home, err := homedir.Dir()
	home := "/etc/netclient"
	file := fmt.Sprintf(home + "/netconfig-" + config.Network)
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
			//config = cfg
		}
	}
}
func ModGlobalConfig(cfg models.IntClient) error{
        var modconfig GlobalConfig
        var err error
        if FileExists("/etc/netclient/netconfig-global-001") {
                useconfig, err := ReadGlobalConfig()
                if err != nil {
                        return err
                }
                modconfig = *useconfig
        }
        if cfg.ServerWGPort != ""{
                modconfig.Client.ServerWGPort = cfg.ServerWGPort
        }
        if cfg.ServerGRPCPort != ""{
                modconfig.Client.ServerGRPCPort = cfg.ServerGRPCPort
        }
        if cfg.ServerAPIPort != ""{
                modconfig.Client.ServerAPIPort = cfg.ServerAPIPort
        }
        if cfg.PublicKey != ""{
                modconfig.Client.PublicKey = cfg.PublicKey
        }
        if cfg.PrivateKey != ""{
                modconfig.Client.PrivateKey = cfg.PrivateKey
        }
        if cfg.ServerPublicEndpoint != ""{
                modconfig.Client.ServerPublicEndpoint = cfg.ServerPublicEndpoint
        }
        if cfg.ServerPrivateAddress != ""{
                modconfig.Client.ServerPrivateAddress = cfg.ServerPrivateAddress
        }
	if cfg.Address != ""{
                modconfig.Client.Address = cfg.Address
        }
        if cfg.Address6 != ""{
                modconfig.Client.Address6 = cfg.Address6
        }
        if cfg.Network != ""{
                modconfig.Client.Network = cfg.Network
        }
        if cfg.ServerKey != ""{
                modconfig.Client.ServerKey = cfg.ServerKey
        }
        if cfg.AccessKey != ""{
                modconfig.Client.AccessKey = cfg.AccessKey
        }
        if cfg.ClientID != ""{
                modconfig.Client.ClientID = cfg.ClientID
        }

        err = WriteGlobal(&modconfig)
        return err
}



func ModConfig(node *nodepb.Node) error{
        network := node.Nodenetwork
        if network == "" {
                return errors.New("No Network Provided")
        }
	var modconfig ClientConfig
	var err error
	if FileExists("/etc/netclient/netconfig-"+network) {
		useconfig, err := ReadConfig(network)
		if err != nil {
			return err
		}
		modconfig = *useconfig
	}
        nodecfg := modconfig.Node
        if node.Name != ""{
                nodecfg.Name = node.Name
        }
        if node.Interface != ""{
                nodecfg.Interface = node.Interface
        }
        if node.Nodenetwork != ""{
                nodecfg.Network = node.Nodenetwork
        }
        if node.Macaddress != ""{
                nodecfg.MacAddress = node.Macaddress
        }
        if node.Localaddress != ""{
                nodecfg.LocalAddress = node.Localaddress
        }
        if node.Postup != ""{
                nodecfg.PostUp = node.Postup
        }
        if node.Postdown != ""{
                nodecfg.PostDown = node.Postdown
        }
        if node.Listenport != 0{
                nodecfg.Port = node.Listenport
        }
        if node.Keepalive != 0{
                nodecfg.KeepAlive = node.Keepalive
        }
        if node.Publickey != ""{
                nodecfg.PublicKey = node.Publickey
        }
        if node.Endpoint != ""{
                nodecfg.Endpoint = node.Endpoint
        }
        if node.Password != ""{
                nodecfg.Password = node.Password
        }
        if node.Address != ""{
                nodecfg.WGAddress = node.Address
        }
        if node.Address6 != ""{
                nodecfg.WGAddress6 = node.Address6
        }
        if node.Postchanges != "" {
                nodecfg.PostChanges = node.Postchanges
        }
        if node.Dnsoff == true {
		nodecfg.DNS = "off"
        }
        if node.Isdualstack == true {
                nodecfg.IsDualStack = "yes"
        }
	if node.Isingressgateway {
		nodecfg.IsIngressGateway = "yes"
	} else {
                nodecfg.IsIngressGateway = "no"
	}
        if node.Localrange != "" && node.Islocal {
                nodecfg.IsLocal = "yes"
                nodecfg.LocalRange = node.Localrange
        }
        modconfig.Node = nodecfg
        err = Write(&modconfig, network)
        return err
}

func GetCLIConfig(c *cli.Context) (ClientConfig, error){
	var cfg ClientConfig
	if c.String("token") != "" {
                tokenbytes, err := base64.StdEncoding.DecodeString(c.String("token"))
                if err  != nil {
			log.Println("error decoding token")
			return cfg, err
                }
		var accesstoken models.AccessToken
		if err := json.Unmarshal(tokenbytes, &accesstoken); err != nil {
			log.Println("error converting token json to object", tokenbytes )
			return cfg, err
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
		cfg.Server.GRPCWireGuard = accesstoken.WG.GRPCWireGuard
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
                if c.String("grpcwg") != "" {
                        cfg.Server.GRPCWireGuard = c.String("grpcwg")
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
	}
	cfg.Node.Name = c.String("name")
	cfg.Node.Interface = c.String("interface")
	cfg.Node.Password = c.String("password")
	cfg.Node.MacAddress = c.String("macaddress")
	cfg.Node.LocalAddress = c.String("localaddress")
	cfg.Node.WGAddress = c.String("address")
	cfg.Node.WGAddress6 = c.String("addressIPV6")
	cfg.Node.Roaming = c.String("roaming")
	cfg.Node.DNS = c.String("dns")
	cfg.Node.IsLocal = c.String("islocal")
	cfg.Node.IsDualStack = c.String("isdualstack")
	cfg.Node.PostUp = c.String("postup")
	cfg.Node.PostDown = c.String("postdown")
	cfg.Node.Port = int32(c.Int("port"))
	cfg.Node.KeepAlive = int32(c.Int("keepalive"))
	cfg.Node.PublicKey = c.String("publickey")
	cfg.Node.PrivateKey = c.String("privatekey")
	cfg.Node.Endpoint = c.String("endpoint")
	cfg.Node.IPForwarding = c.String("ipforwarding")
	cfg.OperatingSystem = c.String("operatingsystem")
	cfg.Daemon = c.String("daemon")

	return cfg, nil
}

func GetCLIConfigRegister(c *cli.Context) (GlobalConfig, error){
	var cfg GlobalConfig
	if c.String("token") != "" {
		tokenbytes, err := base64.StdEncoding.DecodeString(c.String("token"))
		if err != nil {
			log.Println("error decoding token")
			return cfg, err
		}
                var accesstoken models.AccessToken
                if err := json.Unmarshal(tokenbytes, &accesstoken); err != nil {
                        log.Println("error converting token json to object", tokenbytes )
                        return cfg, err
                }
		cfg.Client.ServerPrivateAddress = accesstoken.WG.GRPCWGAddress
		cfg.Client.ServerGRPCPort = accesstoken.WG.GRPCWGPort
		if err != nil {
			log.Println("error decoding token grpcserver")
			return cfg, err
		}
                if err != nil {
                        log.Println("error decoding token apiserver")
                        return cfg, err
                }
                if accesstoken.ServerConfig.APIConnString != "" {
                        cfg.Client.ServerPublicEndpoint = accesstoken.ServerConfig.APIConnString
                } else {
                        cfg.Client.ServerPublicEndpoint = accesstoken.ServerConfig.APIHost
                        if accesstoken.ServerConfig.APIPort != "" {
                                cfg.Client.ServerAPIPort = accesstoken.ServerConfig.APIPort
                        }
                }
		cfg.Client.ServerWGPort = accesstoken.WG.GRPCWGPort
		cfg.Client.ServerKey = accesstoken.ClientConfig.Key
                cfg.Client.ServerKey = accesstoken.WG.GRPCWGPubKey

                if c.String("grpcserver") != "" {
                        cfg.Client.ServerPrivateAddress = c.String("grpcserver")
                }
                if c.String("apiserver") != "" {
                        cfg.Client.ServerPublicEndpoint = c.String("apiserver")
                }
                if c.String("pubkey") != "" {
                        cfg.Client.ServerKey = c.String("pubkey")
                }
                if c.String("network") != "all" {
                        cfg.Client.Network = c.String("network")
                }
        } else {
                cfg.Client.ServerPrivateAddress = c.String("grpcserver")
                cfg.Client.ServerPublicEndpoint = c.String("apiserver")
                cfg.Client.ServerKey = c.String("key")
                cfg.Client.Network = c.String("network")
        }
        cfg.Client.Address = c.String("address")
        cfg.Client.Address6 = c.String("addressIPV6")
        cfg.Client.PublicKey = c.String("pubkey")
        cfg.Client.PrivateKey = c.String("privkey")

        return cfg, nil
}


func ReadConfig(network string) (*ClientConfig, error) {
        if network == "" {
                err := errors.New("No network provided. Exiting.")
                return nil, err
        }
	nofile := false
	home := "/etc/netclient"
	file := fmt.Sprintf(home + "/netconfig-" + network)
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

func ReadGlobalConfig() (*GlobalConfig, error) {
        nofile := false
        home := "/etc/netclient"
        file := fmt.Sprintf(home + "/netconfig-global-001")
        f, err := os.Open(file)

        if err != nil {
                nofile = true
        }
        defer f.Close()

        var cfg GlobalConfig

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
