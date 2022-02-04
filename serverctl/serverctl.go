package serverctl

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/gravitl/netmaker/database"
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
	networks, err := logic.GetNetworks()
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}
	for _, network := range networks {
		node, err := logic.GetNetworkServerLocal(network.NetID)
		if err == nil {
			if err = logic.ServerPull(&node, true); err != nil {
				logger.Log(1, fmt.Sprintf("error pulling server settings for network %s: %s", network.NetID, err.Error()))
			}
		}
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

	ifaceExists := false
	for _, localnet := range localnets {
		if serverNetworkSettings.DefaultInterface == localnet.Name {
			ifaceExists = true
		}
	}

	serverNode, err := logic.GetNetworkServerLocal(network)
	if !ifaceExists && (err == nil && serverNode.ID != "") {
		return logic.ServerUpdate(&serverNode, true)
	} else if !ifaceExists {
		_, err := logic.ServerJoin(&serverNetworkSettings)
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
