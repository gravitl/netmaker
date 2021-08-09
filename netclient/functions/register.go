package functions

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
	//	"github.com/davecgh/go-spew/spew"
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
		AccessKey:  cfg.Client.AccessKey,
		PublicKey:  cfg.Client.PublicKey,
		PrivateKey: cfg.Client.PublicKey,
		Address:    cfg.Client.Address,
		Address6:   cfg.Client.Address6,
		Network:    "comms",
	}

	jsonstring, err := json.Marshal(postclient)
	if err != nil {
		return err
	}
	jsonbytes := []byte(jsonstring)
	body := bytes.NewBuffer(jsonbytes)
	publicaddress := net.JoinHostPort(cfg.Client.ServerPublicEndpoint, cfg.Client.ServerAPIPort)

	res, err := http.Post("http://"+publicaddress+"/api/intclient/register", "application/json", body)
	if err != nil {
		log.Println("Failed to register to http://" + publicaddress + "/api/client/register")
		return err
	}
	if res.StatusCode != http.StatusOK {
		log.Println("Failed to register to http://" + publicaddress + "/api/client/register")
		return errors.New("request to server failed: " + res.Status)
	}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	//bodyString := string(bodyBytes)
	//spew.Dump(bodyString)
	if err != nil {
		return err
	}
	var wgclient models.IntClient
	json.Unmarshal(bodyBytes, &wgclient)
	//spew.Dump(wgclient)
	err = config.ModGlobalConfig(wgclient)
	if err != nil {
		return err
	}
	//spew.Dump(wgclient)
	// err = wireguard.InitGRPCWireguard(wgclient)
	//     if err != nil {
	//             return err
	//     }
	log.Println("registered netclient to " + cfg.Client.ServerPrivateAddress)
	return err
}

func Unregister(cfg config.GlobalConfig) error {
	client := &http.Client{Timeout: 7 * time.Second}
	publicaddress := net.JoinHostPort(cfg.Client.ServerPublicEndpoint, cfg.Client.ServerAPIPort)
	log.Println("sending delete request to: " + "http://" + publicaddress + "/api/intclient/" + cfg.Client.ClientID)
	req, err := http.NewRequest("DELETE", "http://"+publicaddress+"/api/intclient/"+cfg.Client.ClientID, nil)
	if err != nil {
		log.Println(err)
	} else {
		res, err := client.Do(req)
		if res == nil {
			err = errors.New("server not reachable at " + "http://" + publicaddress + "/api/intclient/" + cfg.Client.ClientID)
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
