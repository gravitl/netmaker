package serverctl

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"strings"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
)

// GetServerWGConf - gets the server WG configuration
func GetServerWGConf() (models.IntClient, error) {
	var server models.IntClient
	collection, err := database.FetchRecords(database.INT_CLIENTS_TABLE_NAME)
	if err != nil {
		return models.IntClient{}, errors.New("could not find comms server")
	}
	for _, value := range collection {
		json.Unmarshal([]byte(value), &server)
		if server.Network == "comms" && server.IsServer == "yes" {
			return server, nil
		}
	}
	return models.IntClient{}, errors.New("could not find comms server")
}

// FileExists - checks if local file exists
func FileExists(f string) bool {
	info, err := os.Stat(f)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, errors.New(src + " is not a regular file")
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	err = os.Chmod(dst, 0755)
	if err != nil {
		logic.Log(err.Error(), 1)
	}
	return nBytes, err
}

// RemoveNetwork - removes a network locally on server
func RemoveNetwork(network string) (bool, error) {
	err := logic.ServerLeave(servercfg.GetNodeID(), network)
	return true, err
}

// InitServerNetclient - intializes the server netclient
func InitServerNetclient() error {
	netclientDir := ncutils.GetNetclientPath()
	_, err := os.Stat(netclientDir + "/config")
	if os.IsNotExist(err) {
		os.MkdirAll(netclientDir+"/config", 744)
	} else if err != nil {
		logic.Log("[netmaker] could not find or create "+netclientDir, 1)
		return err
	}
	return nil
}

// HandleContainedClient - function for checkins on server
func HandleContainedClient() error {
	servernets, err := models.GetNetworks()
	if err != nil && !database.IsEmptyRecord(err) {
		return err
	}
	if len(servernets) > 0 {
		if err != nil {
			return err
		}
		log.SetFlags(log.Flags() &^ (log.Llongfile | log.Lshortfile))
		err := SyncNetworks(servernets)
		if err != nil {
			logic.Log("error syncing networks: "+err.Error(), 1)
		}
		for _, serverNet := range servernets {
			err = logic.ServerCheckin(servercfg.GetNodeID(), serverNet.NetID)
			if err != nil {
				logic.Log("error occurred during server checkin: "+err.Error(), 1)
			}
		}
		logic.Log("completed a checkin call", 3)
	}
	return nil
}

// SyncNetworks - syncs the networks for servers
func SyncNetworks(servernets []models.Network) error {

	localnets, err := ncutils.GetSystemNetworks()
	if err != nil {
		return err
	}
	// check networks to join
	for _, servernet := range servernets {
		exists := false
		for _, localnet := range localnets {
			if servernet.NetID == localnet {
				exists = true
			}
		}
		if !exists {
			success, err := AddNetwork(servernet.NetID)
			if err != nil || !success {
				if err == nil {
					err = errors.New("network add failed for " + servernet.NetID)
				}
				if servercfg.GetVerbose() >= 1 {
					if !strings.Contains(err.Error(), "macaddress_unique") { // ignore macaddress unique error throws
						log.Printf("[netmaker] error adding network %s during sync %s \n", servernet.NetID, err)
					}
				}
			}
		}
	}
	// check networks to leave
	for _, localnet := range localnets {
		exists := false
		for _, servernet := range servernets {
			if servernet.NetID == localnet {
				exists = true
			}
		}
		if !exists {
			success, err := RemoveNetwork(localnet)
			if err != nil || !success {
				if err == nil {
					err = errors.New("network delete failed for " + localnet)
				}
				if servercfg.GetVerbose() >= 1 {
					log.Printf("[netmaker] error removing network %s during sync %s \n", localnet, err)
				}
			}
		}
	}
	return nil
}

// AddNetwork - add a network to server in client mode
func AddNetwork(network string) (bool, error) {
	err := logic.ServerJoin(network, servercfg.GetNodeID(), "")
	logic.Log("server added to network "+network, 2)
	return true, err
}
