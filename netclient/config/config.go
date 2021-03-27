package config

import (
//  "github.com/davecgh/go-spew/spew"
  "os"
  "fmt"
  "log"
  "gopkg.in/yaml.v3"
  //homedir "github.com/mitchellh/go-homedir"
)

var Config *ClientConfig

// Configurations exported
type ClientConfig struct {
	Server ServerConfig `yaml:"server"`
	Node NodeConfig `yaml:"node"`
}
type ServerConfig struct {
        Address string `yaml:"address"`
        AccessKey string `yaml:"accesskey"`
}

type NodeConfig struct {
        Name string `yaml:"name"`
        Interface string `yaml:"interface"`
        Group string `yaml:"group"`
        Password string `yaml:"password"`
        MacAddress string `yaml:"macaddress"`
        LocalAddress string `yaml:"localaddress"`
        WGAddress string `yaml:"wgaddress"`
        RoamingOff bool `yaml:"roamingoff"`
        PostUp string `yaml:"postup"`
        PreUp string `yaml:"preup"`
        Port int32 `yaml:"port"`
        KeepAlive int32 `yaml:"keepalive"`
        PublicKey string `yaml:"publickey"`
        PrivateKey string `yaml:"privatekey"`
        Endpoint string `yaml:"endpoint"`
        PostChanges string `yaml:"postchanges"`
}

//reading in the env file
func Write(config *ClientConfig) error{
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
        file := fmt.Sprintf(home + "/.netconfig")
        f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
        if err != nil {
                nofile = true
                //fmt.Println("Could not access " + home + "/.netconfig,  proceeding...")
        }
        defer f.Close()

        if !nofile {
                err = yaml.NewEncoder(f).Encode(config)
                if err != nil {
                        fmt.Println("trouble writing file")
                        return err
                }
        } else {

		newf, err := os.Create(home + "/.netconfig")
		err = yaml.NewEncoder(newf).Encode(config)
		defer newf.Close()
		if err != nil {
			return err
		}
	}


        return err
}
func WriteServer(server string, accesskey string) error{
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

	file := fmt.Sprintf(home + "/.netconfig")
        //f, err := os.Open(file)
        f, err := os.OpenFile(file, os.O_CREATE|os.O_RDWR, 0666)
	//f, err := ioutil.ReadFile(file)
        if err != nil {
		fmt.Println("couldnt open netconfig")
		fmt.Println(err)
                nofile = true
		//err = nil
		return err
        }
        defer f.Close()

	//cfg := &ClientConfig{}
	var cfg ClientConfig

        if !nofile {
		fmt.Println("Writing to existing config file at " + home + "/.netconfig")
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
                fmt.Println("Creating new config file at " + home + "/.netconfig")

                cfg.Server.Address = server
                cfg.Server.AccessKey = accesskey

                newf, err := os.Create(home + "/.netconfig")
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
	file := fmt.Sprintf(home + "/.netconfig")
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


func readConfig() *ClientConfig {
	nofile := false
	//home, err := homedir.Dir()
	home := "/etc/netclient"
	file := fmt.Sprintf(home + "/.netconfig")
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
			log.Fatal(err)
		}
	}
	return &cfg
}

func init() {
  Config = readConfig()
}

