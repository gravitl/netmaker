package functions

import (
	"time"
	"os"
	"log"
	"io/ioutil"
	"bytes"
        "github.com/gravitl/netmaker/netclient/config"
        "github.com/gravitl/netmaker/netclient/local"
        "github.com/gravitl/netmaker/netclient/wireguard"
        "github.com/gravitl/netmaker/models"
	"encoding/json"
	"net/http"
	"errors"
	"github.com/davecgh/go-spew/spew"
)

func Register(cfg config.GlobalConfig) error {

	_, err := os.Stat("/etc/netclient")
        if os.IsNotExist(err) {
                os.Mkdir("/etc/netclient", 744)
        } else if err != nil {
                log.Println("couldnt find or create /etc/netclient")
                return err
        }

        postclient := &models.IntClient{
                AccessKey: cfg.Client.AccessKey,
                PublicKey: cfg.Client.PublicKey,
                PrivateKey: cfg.Client.PublicKey,
		Address: cfg.Client.Address,
		Address6: cfg.Client.Address6,
		Network: "comms",
	}

	jsonstring, err := json.Marshal(postclient)
        if err != nil {
                return err
        }
	jsonbytes := []byte(jsonstring)
	body := bytes.NewBuffer(jsonbytes)
	publicaddress := cfg.Client.ServerPublicEndpoint + ":" + cfg.Client.ServerAPIPort

	log.Println("registering to http://"+publicaddress+"/api/client/register")
	res, err := http.Post("http://"+publicaddress+"/api/intclient/register","application/json",body)
        if err != nil {
                return err
        }
	if res.StatusCode != http.StatusOK {
		return errors.New("request to server failed: " + res.Status)
	}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	var wgclient models.IntClient
	json.Unmarshal(bodyBytes, &wgclient)
        spew.Dump(wgclient)
	err = config.ModGlobalConfig(wgclient)
        if err != nil {
                return err
        }
        spew.Dump(wgclient)
	err = wireguard.InitGRPCWireguard(wgclient)
        if err != nil {
                return err
        }

	return err
}

func Unregister(cfg config.GlobalConfig) error {
	client := &http.Client{ Timeout: 7 * time.Second,}
	publicaddress := cfg.Client.ServerPublicEndpoint + ":" + cfg.Client.ServerAPIPort
	req, err := http.NewRequest("DELETE", "http://"+publicaddress+"/api/intclient/"+cfg.Client.ClientID, nil)
	if err != nil {
                log.Println(err)
        } else {
		res, err := client.Do(req)
		if res == nil {
	                err = errors.New("server not reachable at " + "http://"+publicaddress+"/api/intclient/"+cfg.Client.ClientID)
			log.Println(err)
		} else if res.StatusCode != http.StatusOK {
			err = errors.New("request to server failed: " + res.Status)
	                log.Println(err)
			defer res.Body.Close()
		}
	}
	err = local.WipeGRPCClient()
	if err == nil {
		log.Println("successfully removed grpc client interface")
	}
	return err
}

func Reregister(cfg config.GlobalConfig) error {
	err := Unregister(cfg)
	if err != nil {
		log.Println("failed to un-register")
		return err
	}
	err = Register(cfg)
	if err != nil {
		log.Println("failed to re-register after unregistering")
	}
	return err
}

