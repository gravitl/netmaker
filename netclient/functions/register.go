package functions

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

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
	//create certificate request
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	name := tls.NewCName(os.Getenv("HOSTNAME"))
	csr, err := tls.NewCSR(key, name)
	if err != nil {
		return err
	}
	data := config.RegisterRequest{
		CSR: *csr,
	}
	payload, err := json.Marshal(data)
	url := cfg.Server.API + "/api/server/register"
	log.Println("registering at ", url)

	request, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("authorization", "Bearer "+cfg.Server.AccessKey)
	client := http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return errors.New(response.Status)
	}
	var resp config.RegisterResponse
	if err := json.NewDecoder(response.Body).Decode(&resp); err != nil {
		return errors.New("unmarshal cert error " + err.Error())
	}
	if err := tls.SaveCert(ncutils.GetNetclientPath()+cfg.Server.Server, "root.cert", &resp.CA); err != nil {
		return err
	}
	if err := tls.SaveCert(ncutils.GetNetclientPath(), "client.cert", &resp.Cert); err != nil {
		return err
	}
	if err := tls.SaveKey(ncutils.GetNetclientPath(), "client.key", key); err != nil {
		return err
	}
	logger.Log(0, "certificates/key saved ")
	return nil
}
