package functions

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/daemon"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"github.com/gravitl/netmaker/tls"
	//homedir "github.com/mitchellh/go-homedir"
)

// Pull - pulls the latest config from the server, if manual it will overwrite
func Pull(network string, manual bool) (*models.Node, error) {
	cfg, err := config.ReadConfig(network)
	if err != nil {
		return nil, err
	}
	if cfg.Node.IPForwarding == "yes" && !ncutils.IsWindows() {
		if err = local.SetIPForwarding(); err != nil {
			return nil, err
		}
	}
	token, err := Authenticate(cfg)
	if err != nil {
		return nil, err
	}
	url := "https://" + cfg.Server.API + "/api/nodes/" + cfg.Network + "/" + cfg.Node.ID
	response, err := API("", http.MethodGet, url, token)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		bytes, err := ioutil.ReadAll(response.Body)
		if err != nil {
			fmt.Println(err)
		}
		return nil, (fmt.Errorf("%s %w", string(bytes), err))
	}
	defer response.Body.Close()
	resNode := models.Node{}
	if err := json.NewDecoder(response.Body).Decode(&resNode); err != nil {
		return nil, fmt.Errorf("error decoding node %w", err)
	}
	// ensure that the OS never changes
	resNode.OS = runtime.GOOS
	if manual {
		// check for interface change
		if cfg.Node.Interface != resNode.Interface {
			if err = DeleteInterface(cfg.Node.Interface, cfg.Node.PostDown); err != nil {
				logger.Log(1, "could not delete old interface ", cfg.Node.Interface)
			}
		}
		if err = config.ModConfig(&resNode); err != nil {
			return nil, err
		}
		if err = wireguard.SetWGConfig(network, false); err != nil {
			return nil, err
		}
	} else {
		if err = wireguard.SetWGConfig(network, true); err != nil {
			if errors.Is(err, os.ErrNotExist) && !ncutils.IsFreeBSD() {
				return Pull(network, true)
			} else {
				return nil, err
			}
		}
	}
	var bkupErr = config.SaveBackup(network)
	if bkupErr != nil {
		logger.Log(0, "unable to update backup file")
	}
	//generate new private key and re-register with server
	_, newKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return &resNode, err
	}
	if err := tls.SaveKey(ncutils.GetNetclientPath(), "/client.key", newKey); err != nil {
		return &resNode, err
	}
	if err = RegisterWithServer(&newKey, cfg); err != nil {
		return &resNode, err
	}
	daemon.Restart()

	return &resNode, err
}
