package command

import (
	"log"
	"os"
	"strings"
	"time"

	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/functions"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/netclientutils"
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

	if err != nil {
		if !strings.Contains(err.Error(), "ALREADY_INSTALLED") {
			log.Println("Error installing: ", err)
			err = functions.LeaveNetwork(cfg.Network)
			if err != nil {
				err = local.WipeLocal(cfg.Network)
				if err != nil {
					log.Println("Error removing artifacts: ", err)
				}
			}
			if cfg.Daemon != "off" {
				if !netclientutils.IsWindows() {
					err = local.RemoveSystemDServices(cfg.Network)
				}
				if err != nil {
					log.Println("Error removing services: ", err)
				}
			}
		}
		return err
	}
	log.Println("joined " + cfg.Network)
	if cfg.Daemon != "off" {
		if netclientutils.IsWindows() {
			err = local.CreateAndRunWindowsDaemon()
		} else {
			err = functions.InstallDaemon(cfg)
		}
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
		log.Println("Required, '-n'. No network provided. Exiting.")
		os.Exit(1)
	} else if cfg.Network == "all" {
		log.Println("Running CheckIn for all networks.")
		networks, err := functions.GetNetworks()
		if err != nil {
			log.Println("Error retrieving networks. Exiting.")
			return err
		}
		for _, network := range networks {
			currConf, err := config.ReadConfig(network)
			if err != nil {
				continue
			}
			err = functions.CheckConfig(*currConf)
			if err != nil {
				log.Printf("Error checking in for "+network+" network: ", err)
			} else {
				log.Println("checked in successfully for " + network)
			}
		}
		if len(networks) == 0 {
			if netclientutils.IsWindows() { // Windows specific - there are no netclients, so stop daemon process
				local.StopWindowsDaemon()
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
		log.Println("Error attempting to leave network " + cfg.Network)
	}
	return err
}

func Push(cfg config.ClientConfig) error {
	var err error
	if cfg.Network == "all" || netclientutils.IsWindows() {
		log.Println("No network selected. Running Push for all networks.")
		networks, err := functions.GetNetworks()
		if err != nil {
			log.Println("Error retrieving networks. Exiting.")
			return err
		}
		for _, network := range networks {
			err = functions.Push(network)
			if err != nil {
				log.Printf("Error pushing network configs for "+network+" network: ", err)
			} else {
				log.Println("pushed network config for " + network)
			}
		}
		err = nil
	} else {
		err = functions.Push(cfg.Network)
	}
	log.Println("Completed pushing network configs to remote server.")
	return err
}

func Pull(cfg config.ClientConfig) error {
	var err error
	if cfg.Network == "all" {
		log.Println("No network selected. Running Pull for all networks.")
		networks, err := functions.GetNetworks()
		if err != nil {
			log.Println("Error retrieving networks. Exiting.")
			return err
		}
		for _, network := range networks {
			_, err = functions.Pull(network, true)
			if err != nil {
				log.Printf("Error pulling network config for "+network+" network: ", err)
			} else {
				log.Println("pulled network config for " + network)
			}
		}
		err = nil
	} else {
		_, err = functions.Pull(cfg.Network, true)
	}
	log.Println("Completed pulling network and peer configs.")
	return err
}

func List(cfg config.ClientConfig) error {
	err := functions.List()
	return err
}

func Uninstall() error {
	log.Println("Uninstalling netclient")
	err := functions.Uninstall()
	return err
}
