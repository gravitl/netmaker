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

// Register - the function responsible for registering with the server and acquiring certs
func Register(cfg *config.ClientConfig) error {
	if cfg.Server.Server == "" {
		return errors.New("no server provided")
	}
	if cfg.Server.AccessKey == "" {
		return errors.New("no access key provided")
	}
	//create certificate request
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	//csr, err := tls.NewCSR(private, name)
	if err != nil {
		return err
	}
	data := config.RegisterRequest{
		Key:        private,
		CommonName: tls.NewCName(os.Getenv("HOSTNAME")),
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	url := "https://" + cfg.Server.API + "/api/server/register"
	log.Println("register at ", url)

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

	resp.CA.PublicKey = resp.CAPubKey
	resp.Cert.PublicKey = resp.CertPubKey

	if err := tls.SaveCert(ncutils.GetNetclientPath()+cfg.Server.Server+"/", "root.pem", &resp.CA); err != nil {
		return err
	}
	if err := tls.SaveCert(ncutils.GetNetclientPath()+cfg.Server.Server+"/", "client.pem", &resp.Cert); err != nil {
		return err
	}
	if err := tls.SaveKey(ncutils.GetNetclientPath(), "client.key", private); err != nil {
		return err
	}
	logger.Log(0, "certificates/key saved ")
	return JoinNetwork(cfg, "", false)
}
