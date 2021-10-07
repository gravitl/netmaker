package serverctl

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
	nccommand "github.com/gravitl/netmaker/netclient/command"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/servercfg"
)

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

func InstallNetclient() error {

	netclientPath := ncutils.GetNetclientPath()
	if ncutils.IsWindows() {
		netclientPath += "\\"
	} else {
		netclientPath += "/"
	}
	if !FileExists(netclientPath + "netclient") {
		var err error
		if ncutils.IsWindows() {
			_, err = copy(".\\netclient\\netclient", netclientPath+"netclient")
		} else {
			_, err = copy("./netclient/netclient", netclientPath+"netclient")
		}
		if err != nil {
			log.Println("could not create " + netclientPath + "netclient")
			return err
		}
	}
	return nil
}

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
		log.Println(err)
	}
	return nBytes, err
}

func RemoveNetwork(network string) (bool, error) {
	err := nccommand.Leave(config.ClientConfig{Network: network})
	return true, err
}

func InitServerNetclient() error {
	netclientDir := ncutils.GetNetclientPath()
	_, err := os.Stat(netclientDir + "/config")
	if os.IsNotExist(err) {
		os.MkdirAll(netclientDir+"/config", 744)
	} else if err != nil {
		log.Println("could not find or create", netclientDir)
		return err
	}
	return nil
}

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
		if err != nil && servercfg.GetVerbose() >= 1 {
			log.Printf("[server netclient] error syncing networks %s \n", err)
		}
		err = nccommand.CheckIn(config.ClientConfig{Network: "all"})
		if err != nil && servercfg.GetVerbose() >= 1 {
			log.Printf("[server netclient] error occurred %s \n", err)
		}
		if servercfg.GetVerbose() >= 3 {
			log.Println("[server netclient]", "completed a checkin call")
		}
	}
	return nil
}

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
				log.Printf("[server] error adding network %s during sync %s \n", servernet.NetID, err)
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
				log.Printf("[server] error removing network %s during sync %s \n", localnet, err)
			}
		}
	}
	return nil
}

func AddNetwork(network string) (bool, error) {
	err := nccommand.Join(config.ClientConfig{
		Network: network,
		Daemon:  "off",
		Node: models.Node{
			Network:  network,
			IsServer: "yes",
			Name:     models.NODE_SERVER_NAME,
		},
	}, "")
	log.Println("Server added to network " + network)
	return true, err
}
