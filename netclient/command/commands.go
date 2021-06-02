package command

import (
        "github.com/gravitl/netmaker/netclient/functions"
        "github.com/gravitl/netmaker/netclient/config"
        "github.com/gravitl/netmaker/netclient/local"
        "golang.zx2c4.com/wireguard/wgctrl"
        nodepb "github.com/gravitl/netmaker/grpc"
	"os"
	"strings"
	"log"
)

var (
        wgclient *wgctrl.Client
)

var (
        wcclient nodepb.NodeServiceClient
)

func Register(cfg config.GlobalConfig) error {
        err := functions.Register(cfg)
        return err
}

func Join(cfg config.ClientConfig) error {

	err := functions.JoinNetwork(cfg)
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
	                        err = local.RemoveSystemDServices(cfg.Network)
	                        if err != nil {
	                                log.Println("Error removing services: ", err)
	                        }
			}
		}
		return err
	}
        log.Println("joined " + cfg.Network)
	if cfg.Daemon != "off" {
		err = functions.InstallDaemon(cfg)
	}
	return err
}

func CheckIn(cfg config.ClientConfig) error {
        if cfg.Network == "all" || cfg.Network == "" {
		log.Println("Required, '-n'. No network provided. Exiting.")
                os.Exit(1)
        }
	err := functions.CheckIn(cfg.Network)
	if err != nil {
		log.Println("Error checking in: ", err)
		os.Exit(1)
	}
	return nil
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
        if cfg.Network == "all" {
                log.Println("No network selected. Running Push for all networks.")
                networks, err := functions.GetNetworks()
                if err != nil {
                        log.Println("Error retrieving networks. Exiting.")
                        return err
                }
                for _, network := range networks {
                        err = functions.Push(network)
                        if err != nil {
                                log.Printf("Error pushing network configs for " + network + " network: ", err)
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
			err = functions.Pull(network)
			if err != nil {
				log.Printf("Error pulling network config for " + network + " network: ", err)
			} else {
				log.Println("pulled network config for " + network)
			}
		}
		err = nil
	} else {
	        err = functions.Pull(cfg.Network)
	}
	log.Println("Completed pulling network and peer configs.")
        return err
}

func List(cfg config.ClientConfig) error {
	err := functions.List()
	return err
}

func Uninstall(cfg config.GlobalConfig) error {
	log.Println("Uninstalling netclient")
	err := functions.Uninstall()
	err = functions.Unregister(cfg)
        return err
}
func Unregister(cfg config.GlobalConfig) error {
        err := functions.Unregister(cfg)
        return err
}

