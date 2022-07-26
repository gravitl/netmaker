package functions

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	//homedir "github.com/mitchellh/go-homedir"
)

// Pull - pulls the latest config from the server, if manual it will overwrite
func Pull(network string, iface bool) (*models.Node, error) {
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
		bytes, err := io.ReadAll(response.Body)
		if err != nil {
			fmt.Println(err)
		}
		return nil, (fmt.Errorf("%s %w", string(bytes), err))
	}
	defer response.Body.Close()
	var nodeGET models.NodeGet
	if err := json.NewDecoder(response.Body).Decode(&nodeGET); err != nil {
		return nil, fmt.Errorf("error decoding node %w", err)
	}
	resNode := nodeGET.Node
	// ensure that the OS never changes
	resNode.OS = runtime.GOOS
	if nodeGET.Peers == nil {
		nodeGET.Peers = []wgtypes.PeerConfig{}
	}
	if nodeGET.ServerConfig.API != "" && nodeGET.ServerConfig.MQPort != "" {
		if err = config.ModServerConfig(&nodeGET.ServerConfig, resNode.Network); err != nil {
			logger.Log(0, "unable to update server config: "+err.Error())
		}
	}
	if nodeGET.Node.ListenPort != cfg.Node.LocalListenPort {
		if err := wireguard.RemoveConf(resNode.Interface, false); err != nil {
			logger.Log(0, "error remove interface", resNode.Interface, err.Error())
		}
		err = ncutils.ModPort(&resNode)
		if err != nil {
			return nil, err
		}
		informPortChange(&resNode)
	}
	if err = config.ModNodeConfig(&resNode); err != nil {
		return nil, err
	}
	if iface {
		if err = wireguard.SetWGConfig(network, false, nodeGET.Peers[:]); err != nil {
			return nil, err
		}
	} else {
		if err = wireguard.SetWGConfig(network, true, nodeGET.Peers[:]); err != nil {
			if errors.Is(err, os.ErrNotExist) && !ncutils.IsFreeBSD() {
				return Pull(network, true)
			} else {
				return nil, err
			}
		}
	}
	var bkupErr = config.SaveBackup(network)
	if bkupErr != nil {
		logger.Log(0, "unable to update backup file for", network)
	}

	return &resNode, err
}
