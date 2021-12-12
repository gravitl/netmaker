package command

import (
	"log"
	"os"
	"strconv"
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

	var err error
	err = functions.JoinNetwork(cfg, privateKey)
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
					err = daemon.RemoveSystemDServices()
				}
				if err != nil {
					ncutils.PrintLog("error removing services: "+err.Error(), 1)
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
	if cfg.Daemon != "off" {
		err = daemon.InstallDaemon(cfg)
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

func RunUserspaceDaemon() {

	cfg := config.ClientConfig{
		Network: "all",
	}
	interval := getWindowsInterval()
	dur := time.Duration(interval) * time.Second
	for {
		if err := CheckIn(cfg); err != nil {
			// pass
		}
		time.Sleep(dur)
	}
}

func CheckIn(cfg config.ClientConfig) error {
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
		networks, err := ncutils.GetSystemNetworks()
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
		networks, err := ncutils.GetSystemNetworks()
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
	err := functions.List(cfg.Network)
	return err
}

func Uninstall() error {
	ncutils.PrintLog("uninstalling netclient", 0)
	err := functions.Uninstall()
	return err
}
