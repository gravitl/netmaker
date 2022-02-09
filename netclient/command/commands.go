package command

import (
	"os"
	"strconv"
	"strings"
	"time"

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
	if ncutils.IsWindows() {
		ncutils.PrintLog("setting up WireGuard app", 0)
		time.Sleep(time.Second >> 1)
		functions.Pull(cfg.Network, true)
	}
	return err
}

func getWindowsInterval() int {
	interval := 15
	networks, err := ncutils.GetSystemNetworks()
	if err != nil {
		return interval
	}
	cfg, err := config.ReadConfig(networks[0])
	if err != nil {
		return interval
	}
	netint, err := strconv.Atoi(cfg.Server.CheckinInterval)
	if err == nil && netint != 0 {
		interval = netint
	}
	return interval
}

// RunUserspaceDaemon - runs continual checkins
func RunUserspaceDaemon() {

	cfg := config.ClientConfig{
		Network: "all",
	}
	interval := getWindowsInterval()
	dur := time.Duration(interval) * time.Second
	for {
		CheckIn(cfg)
		time.Sleep(dur)
	}
}

// CheckIn - runs checkin command from cli
func CheckIn(cfg config.ClientConfig) error {
	//log.Println("checkin --- diabled for now")
	//return nil
	var err error
	var errN error
	if cfg.Network == "" {
		ncutils.PrintLog("required, '-n', exiting", 0)
		os.Exit(1)
	} else if cfg.Network == "all" {
		ncutils.PrintLog("running checkin for all networks", 1)
		networks, err := ncutils.GetSystemNetworks()
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
				if strings.Contains(err.Error(), "could not find iface") {
					err = Pull(cfg)
					if err != nil {
						ncutils.PrintLog(err.Error(), 1)
					}
				} else {
					ncutils.PrintLog("error checking in for "+network+" network: "+err.Error(), 1)
				}
			} else {
				ncutils.PrintLog("checked in successfully for "+network, 1)
			}
		}
		if len(networks) == 0 {
			if ncutils.IsWindows() { // Windows specific - there are no netclients, so stop daemon process
				daemon.StopWindowsDaemon()
			}
		}
		errN = err
		err = nil
	} else {
		err = functions.CheckConfig(cfg)
	}
	if err == nil && errN != nil {
		err = errN
	}
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

// Push - runs push command
func Push(cfg config.ClientConfig) error {
	var err error
	if cfg.Network == "all" || ncutils.IsWindows() {
		ncutils.PrintLog("pushing config to server for all networks.", 0)
		networks, err := ncutils.GetSystemNetworks()
		if err != nil {
			ncutils.PrintLog("error retrieving networks, exiting.", 0)
			return err
		}
		for _, network := range networks {
			err = functions.Push(network)
			if err != nil {
				ncutils.PrintLog("error pushing network configs for network: "+network+"\n"+err.Error(), 1)
			} else {
				ncutils.PrintLog("pushed network config for "+network, 1)
			}
		}
		err = nil
	} else {
		err = functions.Push(cfg.Network)
	}
	if err == nil {
		ncutils.PrintLog("completed pushing network configs to remote server", 1)
		ncutils.PrintLog("success", 1)
	} else {
		ncutils.PrintLog("error occurred pushing configs", 1)
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
