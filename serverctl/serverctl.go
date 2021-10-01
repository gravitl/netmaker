package serverctl

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/functions"
	"github.com/gravitl/netmaker/models"
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

	netclientPath := ncutils.GetNetclientPathSpecific()
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
	netclientPath := ncutils.GetNetclientPathSpecific()
	_, err := os.Stat(netclientPath + "netclient")
	if err != nil {
		log.Println("could not find " + netclientPath + "netclient")
		return false, err
	}
	_, err = ncutils.RunCmd(netclientPath+"netclient leave -n "+network, true)
	if err == nil {
		log.Println("Server removed from network " + network)
	}
	return true, err

}

func InitServerNetclient() error {
	netclientDir := ncutils.GetNetclientPath()
	netclientPath := ncutils.GetNetclientPathSpecific()
	_, err := os.Stat(netclientDir)
	if os.IsNotExist(err) {
		os.Mkdir(netclientDir, 744)
	} else if err != nil {
		log.Println("could not find or create", netclientDir)
		return err
	}
	_, err = os.Stat(netclientPath + "netclient")
	if os.IsNotExist(err) {
		err = InstallNetclient()
		if err != nil {
			return err
		}
	}
	err = os.Chmod(netclientPath+"netclient", 0755)
	if err != nil {
		log.Println("could not change netclient binary permissions")
		return err
	}
	return nil
}

func HandleContainedClient() error {
	log.SetFlags(log.Flags() &^ (log.Llongfile | log.Lshortfile))

	netclientPath := ncutils.GetNetclientPathSpecific()
	checkinCMD := exec.Command(netclientPath+"netclient", "checkin", "-n", "all")
	if servercfg.GetVerbose() >= 2 {
		checkinCMD.Stdout = os.Stdout
	}
	checkinCMD.Stderr = os.Stderr
	err := checkinCMD.Start()
	if err != nil {
		if servercfg.GetVerbose() >= 2 {
			log.Println(err)
		}
	}
	err = checkinCMD.Wait()
	if err != nil {
		if servercfg.GetVerbose() >= 2 {
			log.Println(err)
		}
	}
	if servercfg.GetVerbose() >= 3 {
		log.Println("[server netclient]", "completed a checkin call")
	}
	return nil
}

func AddNetwork(network string) (bool, error) {
	pubip, err := servercfg.GetPublicIP()
	if err != nil {
		log.Println("could not get public IP.")
		return false, err
	}
	netclientPath := ncutils.GetNetclientPathSpecific()

	token, err := functions.CreateServerToken(network)
	if err != nil {
		log.Println("could not create server token for " + network)
		return false, err
	}

	functions.PrintUserLog(models.NODE_SERVER_NAME, "executing network join: "+netclientPath+"netclient "+"join "+"-t "+token+" -name "+models.NODE_SERVER_NAME+" -endpoint "+pubip, 0)
	var joinCMD *exec.Cmd
	if servercfg.IsClientMode() == "contained" {
		joinCMD = exec.Command(netclientPath+"netclient", "join", "-t", token, "-name", models.NODE_SERVER_NAME, "-endpoint", pubip, "-daemon", "off", "-dnson", "no")
	} else {
		joinCMD = exec.Command(netclientPath+"netclient", "join", "-t", token, "-name", models.NODE_SERVER_NAME, "-endpoint", pubip)
	}
	joinCMD.Stdout = os.Stdout
	joinCMD.Stderr = os.Stderr
	err = joinCMD.Start()

	if err != nil {
		log.Println(err)
	}
	log.Println("Waiting for join command to finish...")
	err = joinCMD.Wait()
	if err != nil {
		log.Printf("Command finished with error: %v", err)
		return false, err
	}
	log.Println("Server added to network " + network)
	return true, err
}
