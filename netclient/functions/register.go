package functions

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/tls"
)

func Register(cfg *config.ClientConfig) error {
	if cfg.Server.Server == "" {
		return errors.New("no server provided")
	}
	if cfg.Server.AccessKey == "" {
		return errors.New("no access key provided")
	}
	url := cfg.Server.API + "/api/server/register"
	log.Println("regsiter at ", url)
	request, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("authorization", "Bearer "+cfg.Server.AccessKey)
	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return errors.New(response.Status)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)

	}
	log.Println(string(body))
	var cert *x509.Certificate
	if err := json.Unmarshal(body, cert); err != nil {
		//if err := json.NewDecoder(response.Body).Decode(cert); err != nil {
		return errors.New("unmarshal cert error " + err.Error())
	}
	if err := tls.SaveCert(ncutils.GetNetclientPath()+cfg.Server.Server, "root.cert", cert); err != nil {
		return err
	}
	logger.Log(0, "server certificate saved ")
	return nil
}
