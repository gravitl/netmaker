package command

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/daemon"
	"github.com/gravitl/netmaker/netclient/functions"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// Join - join command to run from cli
func Join(cfg *config.ClientConfig, privateKey string) error {
	var err error
	//join network
	if cfg.SsoServer != "" {
		// User wants to get access key from the OIDC server
		// Do that before the Joining Network flow by performing the end point auth flow
		// if performed successfully an access key is obtained from the server and then we
		// proceed with the usual flow 'pretending' that user is feeded us with an access token
		if len(cfg.Network) == 0 || cfg.Network == "all" {
			return fmt.Errorf("no network provided. Specify network with \"-n <net name>\"")
		}
		logger.Log(1, "Logging into %s via:", cfg.Network, cfg.SsoServer)
		err = functions.JoinViaSSo(cfg, privateKey)
		if err != nil {
			logger.Log(0, "Join failed: ", err.Error())
			return err
		}

		if cfg.AccessKey == "" {
			return errors.New("login failed")
		}
	}

	logger.Log(1, "Joining network: ", cfg.Network)
	err = functions.JoinNetwork(cfg, privateKey)
	if err != nil {
		if !strings.Contains(err.Error(), "ALREADY_INSTALLED") {
			logger.Log(0, "error installing: ", err.Error())
			err = functions.WipeLocal(cfg)
			if err != nil {
				logger.Log(1, "error removing artifacts: ", err.Error())
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
		}
		if err != nil && strings.Contains(err.Error(), "ALREADY_INSTALLED") {
			logger.Log(0, err.Error())
			err = nil
		}
		return err
	}
	logger.Log(1, "joined", cfg.Network)

	return err
}

// Leave - runs the leave command from cli
func Leave(cfg *config.ClientConfig) error {
	err := functions.LeaveNetwork(cfg.Network)
	if err != nil {
		logger.Log(1, "error attempting to leave network "+cfg.Network)
	} else {
		logger.Log(0, "success")
	}
	return err
}

// Pull - runs pull command from cli
func Pull(cfg *config.ClientConfig) error {
	var err error
	var networks = []string{}
	if cfg.Network == "all" {
		logger.Log(0, "No network selected. Running Pull for all networks.")
		networks, err = ncutils.GetSystemNetworks()
		if err != nil {
			logger.Log(1, "Error retrieving networks. Exiting.")
			return err
		}
	} else {
		networks = append(networks, cfg.Network)
	}

	var currentServers = make(map[string]config.ClientConfig)

	for _, network := range networks {
		currCfg, err := config.ReadConfig(network)
		if err != nil {
			logger.Log(1, "could not read config when pulling for network", network)
			continue
		}

		_, err = functions.Pull(network, true)
		if err != nil {
			logger.Log(1, "error pulling network config for network: ", network, "\n", err.Error())
		} else {
			logger.Log(1, "pulled network config for "+network)
		}

		currentServers[currCfg.Server.Server] = *currCfg
	}
	daemon.Restart()
	logger.Log(1, "reset network", cfg.Network, "and peer configs")
	return err
}

// List - runs list command from cli
func List(cfg config.ClientConfig) error {
	_, err := functions.List(cfg.Network)
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

// Install - installs binary and daemon
func Install() error {
	return functions.Install()
}

// Connect - re-instates a connection of a node
func Connect(cfg config.ClientConfig) error {
	networkName := cfg.Network
	if networkName == "" {
		networkName = cfg.Node.Network
	}
	if networkName == "all" {
		return fmt.Errorf("no network specified")
	}
	return functions.Connect(networkName)
}

// Disconnect - disconnects a connection of a node
func Disconnect(cfg config.ClientConfig) error {
	networkName := cfg.Network
	if networkName == "" {
		networkName = cfg.Node.Network
	}
	if networkName == "all" {
		return fmt.Errorf("no network specified")
	}
	return functions.Disconnect(networkName)
}
