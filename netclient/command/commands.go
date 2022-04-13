package command

import (
	"strings"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/daemon"
	"github.com/gravitl/netmaker/netclient/functions"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// JoinComms -- Join the message queue comms network if it doesn't have it
// tries to ping if already found locally, if fail ping pull for best effort for communication
func JoinComms(cfg *config.ClientConfig) error {
	commsCfg := &config.ClientConfig{}
	commsCfg.Network = cfg.Server.CommsNetwork
	commsCfg.Node.Network = cfg.Server.CommsNetwork
	commsCfg.Server.AccessKey = cfg.Server.AccessKey
	commsCfg.Server.GRPCAddress = cfg.Server.GRPCAddress
	commsCfg.Server.GRPCSSL = cfg.Server.GRPCSSL
	commsCfg.Server.CoreDNSAddr = cfg.Server.CoreDNSAddr
	if commsCfg.ConfigFileExists() {
		return nil
	}
	commsCfg.ReadConfig()

	if len(commsCfg.Node.Name) == 0 {
		if err := functions.JoinNetwork(commsCfg, "", true); err != nil {
			return err
		}
	} else { // check if comms is currently reachable
		if err := functions.PingServer(commsCfg); err != nil {
			if err = Pull(commsCfg); err != nil {
				return err
			}
		}
	}
	return nil
}

// Join - join command to run from cli
func Join(cfg *config.ClientConfig, privateKey string) error {
	var err error
	//join network
	err = functions.JoinNetwork(cfg, privateKey, false)
	if err != nil && !cfg.DebugOn {
		if !strings.Contains(err.Error(), "ALREADY_INSTALLED") {
			logger.Log(1, "error installing: ", err.Error())
			err = functions.LeaveNetwork(cfg.Network, true)
			if err != nil {
				err = functions.WipeLocal(cfg.Network)
				if err != nil {
					logger.Log(1, "error removing artifacts: ", err.Error())
				}
			}
			if cfg.Daemon != "off" {
				if ncutils.IsLinux() {
					err = daemon.RemoveSystemDServices()
				}
				if err != nil {
					logger.Log(1, "error removing services: ", err.Error())
				}
				if ncutils.IsFreeBSD() {
					daemon.RemoveFreebsdDaemon()
				}
			}
		} else {
			logger.Log(0, "success")
		}
		if err != nil && strings.Contains(err.Error(), "ALREADY_INSTALLED") {
			logger.Log(0, err.Error())
			err = nil
		}
		return err
	}
	logger.Log(1, "joined ", cfg.Network)
	/*
		if ncutils.IsWindows() {
			logger.Log("setting up WireGuard app", 0)
			time.Sleep(time.Second >> 1)
			functions.Pull(cfg.Network, true)
		}
	*/
	return err
}

// Leave - runs the leave command from cli
func Leave(cfg *config.ClientConfig, force bool) error {
	err := functions.LeaveNetwork(cfg.Network, force)
	if err != nil {
		logger.Log(1, "error attempting to leave network "+cfg.Network)
	} else {
		logger.Log(0, "success")
	}
	//nets, err := ncutils.GetSystemNetworks()
	//if err == nil && len(nets) == 1 {
	//if nets[0] == cfg.Node.CommID {
	//logger.Log(1, "detected comms as remaining network, removing...")
	//err = functions.LeaveNetwork(nets[0], true)
	//}
	//}
	return err
}

// Pull - runs pull command from cli
func Pull(cfg *config.ClientConfig) error {
	var err error
	if cfg.Network == "all" {
		logger.Log(0, "No network selected. Running Pull for all networks.")
		networks, err := ncutils.GetSystemNetworks()
		if err != nil {
			logger.Log(1, "Error retrieving networks. Exiting.")
			return err
		}
		for _, network := range networks {
			_, err = functions.Pull(network, true)
			if err != nil {
				logger.Log(1, "Error pulling network config for network: ", network, "\n", err.Error())
			} else {
				logger.Log(1, "pulled network config for "+network)
			}
		}
		err = nil
	} else {
		_, err = functions.Pull(cfg.Network, true)
	}
	logger.Log(1, "reset network and peer configs")
	if err == nil {
		logger.Log(1, "reset network and peer configs")
		logger.Log(1, "success")
	} else {
		logger.Log(0, "error occurred pulling configs from server")
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
	logger.Log(0, "uninstalling netclient...")
	err := functions.Uninstall()
	logger.Log(0, "uninstalled netclient")
	return err
}

// Daemon - runs the daemon
func Daemon() error {
	err := functions.Daemon()
	return err
}

func Register(cfg *config.ClientConfig) error {
	return functions.Register(cfg)
}
