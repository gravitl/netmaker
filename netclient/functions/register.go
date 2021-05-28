package functions

import (
	"log"
	"io/ioutil"
	"bytes"
        "github.com/gravitl/netmaker/netclient/config"
        "github.com/gravitl/netmaker/netclient/wireguard"
        "github.com/gravitl/netmaker/models"
	"encoding/json"
	"net/http"
	"errors"
	"github.com/davecgh/go-spew/spew"
)

func Register(cfg config.GlobalConfig) error {

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
	log.Println(jsonstring)
	log.Println("http://"+cfg.Client.ServerEndpoint+"/api/client/register","application/json")
	res, err := http.Post("http://"+cfg.Client.ServerEndpoint+"/api/intclient/register","application/json",body)
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

	err = wireguard.InitGRPCWireguard(wgclient)
        if err != nil {
                return err
        }

	return err
}
