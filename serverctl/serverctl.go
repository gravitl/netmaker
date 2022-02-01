package serverctl

import (
	"errors"
	"net"
	"os"
	"strings"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

const NETMAKER_BINARY_NAME = "netmaker"

// InitServerNetclient - intializes the server netclient
func InitServerNetclient() error {
	netclientDir := ncutils.GetNetclientPath()
	_, err := os.Stat(netclientDir + "/config")
	if os.IsNotExist(err) {
		os.MkdirAll(netclientDir+"/config", 0744)
	} else if err != nil {
		logger.Log(1, "could not find or create", netclientDir)
		return err
	}
	return nil
}

// SyncServerNetwork - ensures a wg interface and node exists for server
func SyncServerNetwork(network string) error {
	serverNetworkSettings, err := logic.GetNetwork(network)
	if err != nil {
		return err
	}
	localnets, err := net.Interfaces()
	if err != nil {
		return err
	}
	exists := false
	for _, localnet := range localnets {
		if serverNetworkSettings.DefaultInterface == localnet.Name {
			exists = true
		}
	}
	// add networks locally that exist in database
	if !exists {
		err := logic.ServerJoin(&serverNetworkSettings)
		if err != nil {
			if err == nil {
				err = errors.New("network add failed for " + serverNetworkSettings.NetID)
			}
			if !strings.Contains(err.Error(), "macaddress_unique") { // ignore macaddress unique error throws
				logger.Log(1, "error adding network", serverNetworkSettings.NetID, "during sync:", err.Error())
			}
		}
	}

	// remove networks locally that do not exist in database
	/*
		for _, localnet := range localnets {
			if strings.Contains(localnet.Name, "nm-") {
				var exists = ""
				if serverNetworkSettings.DefaultInterface == localnet.Name {
					exists = serverNetworkSettings.NetID
				}
				if exists == "" {
					err := logic.DeleteNodeByID(serverNode, true)
					if err != nil {
						if err == nil {
							err = errors.New("network delete failed for " + exists)
						}
						logger.Log(1, "error removing network", exists, "during sync", err.Error())
					}
				}
			}
		}
	*/
	return nil
}
