package command

import (
	"strings"

	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/daemon"
	"github.com/gravitl/netmaker/netclient/functions"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// Join - join command to run from cli
func Join(cfg config.ClientConfig, privateKey string) error {

	var err error
	err = functions.JoinNetwork(cfg, privateKey)
	if err != nil && !cfg.DebugOn {
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
					err = daemon.RemoveSystemDServices()
				}
				if err != nil {
					ncutils.PrintLog("error removing services: "+err.Error(), 1)
				}
				if ncutils.IsFreeBSD() {
					daemon.RemoveFreebsdDaemon()
				}
			}
		} else {
			ncutils.PrintLog("success", 0)
		}
		if err != nil && strings.Contains(err.Error(), "ALREADY_INSTALLED") {
			ncutils.PrintLog(err.Error(), 0)
			err = nil
		}
		return err
	}
	ncutils.PrintLog("joined "+cfg.Network, 1)
	/*
		if ncutils.IsWindows() {
			ncutils.PrintLog("setting up WireGuard app", 0)
			time.Sleep(time.Second >> 1)
			functions.Pull(cfg.Network, true)
		}
	*/
	return err
}

// Leave - runs the leave command from cli
func Leave(cfg config.ClientConfig) error {
	err := functions.LeaveNetwork(cfg.Network)
	if err != nil {
		ncutils.PrintLog("error attempting to leave network "+cfg.Network, 1)
	} else {
		ncutils.PrintLog("success", 0)
	}
	return err
}

// Pull - runs pull command from cli
func Pull(cfg config.ClientConfig) error {
	var err error
	if cfg.Network == "all" {
		ncutils.PrintLog("No network selected. Running Pull for all networks.", 0)
		networks, err := ncutils.GetSystemNetworks()
		if err != nil {
			ncutils.PrintLog("Error retrieving networks. Exiting.", 1)
			return err
		}
		for _, network := range networks {
			_, err = functions.Pull(network, true)
			if err != nil {
				ncutils.PrintLog("Error pulling network config for network: "+network+"\n"+err.Error(), 1)
			} else {
				ncutils.PrintLog("pulled network config for "+network, 1)
			}
		}
		err = nil
	} else {
		_, err = functions.Pull(cfg.Network, true)
	}
	ncutils.PrintLog("reset network and peer configs", 1)
	if err == nil {
		ncutils.PrintLog("reset network and peer configs", 1)
		ncutils.PrintLog("success", 1)
	} else {
		ncutils.PrintLog("error occurred pulling configs from server", 1)
	}
	return err
}

// List - runs list command from cli
func List(cfg config.ClientConfig) error {
	err := functions.List(cfg.Network)
	return err
}

// Uninstall - runs uninstall command from cli
func Uninstall() error {
	ncutils.PrintLog("uninstalling netclient...", 0)
	err := functions.Uninstall()
	ncutils.PrintLog("uninstalled netclient", 0)
	return err
}

func Daemon() error {
	err := functions.Daemon()
	return err
}
