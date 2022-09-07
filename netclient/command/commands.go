package command

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"strings"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/daemon"
	"github.com/gravitl/netmaker/netclient/functions"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/tls"
)

// Join - join command to run from cli
func Join(cfg *config.ClientConfig, privateKey string) error {
	var err error
	//join network
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
	//generate new client key if one doesn' exist
	var private *ed25519.PrivateKey
	private, err = tls.ReadKeyFromFile(ncutils.GetNetclientPath() + ncutils.GetSeparator() + "client.key")
	if err != nil {
		_, newKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		if err := tls.SaveKeyToFile(ncutils.GetNetclientPath(), ncutils.GetSeparator()+"client.key", newKey); err != nil {
			return err
		}
		private = &newKey
	}
	// re-register with server -- get new certs for broker
	for _, clientCfg := range currentServers {
		if err = functions.RegisterWithServer(private, &clientCfg); err != nil {
			logger.Log(0, "registration error", err.Error())
		} else {
			daemon.Restart()
		}
	}
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
