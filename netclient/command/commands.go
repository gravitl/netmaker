package command

import (
	"log"
	"os"
	"strings"
	"time"

	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/daemon"
	"github.com/gravitl/netmaker/netclient/functions"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.zx2c4.com/wireguard/wgctrl"
)

var (
	wgclient *wgctrl.Client
)

var (
	wcclient nodepb.NodeServiceClient
)

func Join(cfg config.ClientConfig, privateKey string) error {

	err := functions.JoinNetwork(cfg, privateKey)

	if err != nil && !cfg.DebugJoin {
		if !strings.Contains(err.Error(), "ALREADY_INSTALLED") {
			ncutils.PrintLog("error installing: "+err.Error(), 1)
			err = functions.LeaveNetwork(cfg.Network)
			if err != nil {
				err = functions.WipeLocal(cfg.Network)
				if err != nil {
					ncutils.PrintLog("error removing artifacts: "+err.Error(), 1)
				}
			}
			if cfg.Daemon != "off" {
				if ncutils.IsLinux() {
					err = daemon.RemoveSystemDServices(cfg.Network)
				}
				if err != nil {
					ncutils.PrintLog("error removing services: "+err.Error(), 1)
				}
			}
		} else {
			ncutils.PrintLog("success", 0)
		}
		return err
	}
	ncutils.PrintLog("joined "+cfg.Network, 1)
	if cfg.Daemon != "off" {
		err = daemon.InstallDaemon(cfg)
	}
	return err
}

func RunUserspaceDaemon() {
	cfg := config.ClientConfig{
		Network: "all",
	}
	for {
		if err := CheckIn(cfg); err != nil {
			// pass
		}
		time.Sleep(30 * time.Second)
	}
}

func CheckIn(cfg config.ClientConfig) error {
	var err error
	if cfg.Network == "" {
		ncutils.PrintLog("required, '-n', exiting", 0)
		os.Exit(1)
	} else if cfg.Network == "all" {
		ncutils.PrintLog("running checkin for all networks", 1)
		networks, err := functions.GetNetworks()
		if err != nil {
			ncutils.PrintLog("error retrieving networks, exiting", 1)
			return err
		}
		for _, network := range networks {
			currConf, err := config.ReadConfig(network)
			if err != nil {
				continue
			}
			err = functions.CheckConfig(*currConf)
			if err != nil {
				ncutils.PrintLog("error checking in for "+network+" network: "+err.Error(), 1)
			} else {
				ncutils.PrintLog("checked in successfully for "+network, 1)
			}
		}
		if len(networks) == 0 {
			if ncutils.IsWindows() { // Windows specific - there are no netclients, so stop daemon process
				daemon.StopWindowsDaemon()
			}
		}
		err = nil
	} else {
		err = functions.CheckConfig(cfg)
	}
	return err
}

func Leave(cfg config.ClientConfig) error {
	err := functions.LeaveNetwork(cfg.Network)
	if err != nil {
		ncutils.PrintLog("error attempting to leave network "+cfg.Network, 1)
	} else {
		ncutils.PrintLog("success", 0)
	}
	return err
}

func Push(cfg config.ClientConfig) error {
	var err error
	if cfg.Network == "all" || ncutils.IsWindows() {
		ncutils.PrintLog("pushing config to server for all networks.", 0)
		networks, err := functions.GetNetworks()
		if err != nil {
			ncutils.PrintLog("error retrieving networks, exiting.", 0)
			return err
		}
		for _, network := range networks {
			err = functions.Push(network)
			if err != nil {
				log.Printf("error pushing network configs for "+network+" network: ", err)
			} else {
				ncutils.PrintLog("pushed network config for "+network, 1)
			}
		}
		err = nil
	} else {
		err = functions.Push(cfg.Network)
	}
	ncutils.PrintLog("completed pushing network configs to remote server", 1)
	ncutils.PrintLog("success", 1)
	return err
}

func Pull(cfg config.ClientConfig) error {
	var err error
	if cfg.Network == "all" {
		ncutils.PrintLog("No network selected. Running Pull for all networks.", 0)
		networks, err := functions.GetNetworks()
		if err != nil {
			ncutils.PrintLog("Error retrieving networks. Exiting.", 1)
			return err
		}
		for _, network := range networks {
			_, err = functions.Pull(network, true)
			if err != nil {
				log.Printf("Error pulling network config for "+network+" network: ", err)
			} else {
				ncutils.PrintLog("pulled network config for "+network, 1)
			}
		}
		err = nil
	} else {
		_, err = functions.Pull(cfg.Network, true)
	}
	ncutils.PrintLog("reset network and peer configs", 1)
	ncutils.PrintLog("success", 1)
	return err
}

func List(cfg config.ClientConfig) error {
	err := functions.List()
	return err
}

func Uninstall() error {
	ncutils.PrintLog("uninstalling netclient", 0)
	err := functions.Uninstall()
	return err
}
