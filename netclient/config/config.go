package config

import (
//  "github.com/davecgh/go-spew/spew"
  "os"
  "errors"
  "fmt"
  "log"
  "gopkg.in/yaml.v3"
  //homedir "github.com/mitchellh/go-homedir"
)

//var Config *ClientConfig

// Configurations exported
type ClientConfig struct {
	Server ServerConfig `yaml:"server"`
	Node NodeConfig `yaml:"node"`
	Network string
}
type ServerConfig struct {
        Address string `yaml:"address"`
        AccessKey string `yaml:"accesskey"`
}

type NodeConfig struct {
        Name string `yaml:"name"`
        Interface string `yaml:"interface"`
        Network string `yaml:"network"`
        Password string `yaml:"password"`
        MacAddress string `yaml:"macaddress"`
        LocalAddress string `yaml:"localaddress"`
        WGAddress string `yaml:"wgaddress"`
        RoamingOff bool `yaml:"roamingoff"`
        IsLocal bool `yaml:"islocal"`
        AllowedIPs string `yaml:"allowedips"`
        LocalRange string `yaml:"localrange"`
        PostUp string `yaml:"postup"`
        PostDown string `yaml:"postdown"`
        Port int32 `yaml:"port"`
        KeepAlive int32 `yaml:"keepalive"`
        PublicKey string `yaml:"publickey"`
        PrivateKey string `yaml:"privatekey"`
        Endpoint string `yaml:"endpoint"`
        PostChanges string `yaml:"postchanges"`
}

//reading in the env file
func Write(config *ClientConfig, network string) error{
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
                return err
        }
	home := "/etc/netclient"

        if err != nil {
                log.Fatal(err)
        }
        file := fmt.Sprintf(home + "/netconfig-" + network)
        f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
        if err != nil {
                nofile = true
                //fmt.Println("Could not access " + home + "/netconfig,  proceeding...")
        }
        defer f.Close()

        if !nofile {
                err = yaml.NewEncoder(f).Encode(config)
                if err != nil {
                        fmt.Println("trouble writing file")
                        return err
                }
        } else {

		newf, err := os.Create(home + "/netconfig-" + network)
		err = yaml.NewEncoder(newf).Encode(config)
		defer newf.Close()
		if err != nil {
			return err
		}
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

		cfg.Server.Address = server
		cfg.Server.AccessKey = accesskey

		err = yaml.NewEncoder(f).Encode(cfg)
		//_, err = yaml.Marshal(f, &cfg)
		if err != nil {
                        fmt.Println("trouble encoding file")
                        return err
                }
	} else {
                fmt.Println("Creating new config file at " + home + "/netconfig-" + network)

                cfg.Server.Address = server
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

func ReadConfig(network string) (*ClientConfig, error) {
        if network == "" {
                err := errors.New("No network provided. Exiting.")
                return nil, err
        }
	nofile := false
	//home, err := homedir.Dir()
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
/*
func init() {
  Config = readConfig()
}
*/

