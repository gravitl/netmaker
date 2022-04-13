package functions

import (
	"crypto/x509"
	"encoding/json"
	"errors"
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
	url := "https://" + cfg.Server.Server + "/api/register"
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
	var cert *x509.Certificate
	if err := json.NewDecoder(response.Body).Decode(cert); err != nil {
		return err
	}
	if err := tls.SaveCert(ncutils.GetNetclientPath()+cfg.Server.Server, "root.cert", cert); err != nil {
		return err
	}
	logger.Log(0, "server certificate saved ")
	return nil
}
