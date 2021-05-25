package command

import (
        "github.com/gravitl/netmaker/netclient/functions"
        "github.com/gravitl/netmaker/models"
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

func Register(cfg config.ClientConfig) error {

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
                        err = local.RemoveSystemDServices(cfg.Network)
                        if err != nil {
                                log.Println("Error removing services: ", err)
                        }
		}
		os.Exit(1)
		} else {
			log.Println(err.Error())
			os.Exit(1)
		}
	}
        log.Println("joined " + cfg.Network)
	if cfg.Daemon != "off" {
		err = functions.Install(cfg)
	        log.Println("installed daemon")
	}
	return err
}

func CheckIn(cfg config.ClientConfig) error {
                        if cfg.Network == "nonetwork" || cfg.Network == "" {
                                log.Println("Required, '-n'. No network provided. Exiting.")
                                os.Exit(1)
                        }
			log.Println("Beginning node check in for network " + cfg.Network)
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
	log.Println("pushing to network")
	return nil
}

func Pull(cfg config.ClientConfig) error {
        log.Println("pulling from network")
        return nil
}

func Status(cfg config.ClientConfig) error {
        log.Println("retrieving network status")
        return nil
}

func Uninstall(cfg config.ClientConfig) error {
        log.Println("uninstalling")
        return nil
}
