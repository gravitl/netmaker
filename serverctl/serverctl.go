package serverctl

import (
        "fmt"
	"net/http"
	"io/ioutil"
	"github.com/gravitl/netmaker/functions"
	"io"
	"errors"
        "os"
        "os/exec"
)

func CreateDefaultNetwork() (bool, error) {

        fmt.Println("Creating default network...")

        iscreated := false
        exists, err := functions.NetworkExists(config.Config.Server.DefaultNetName)

        if exists || err != nil {
                fmt.Println("Default network already exists. Skipping...")
                return iscreated, err
        } else {

        var network models.Network

        network.NetID = "default"
        network.AddressRange = "10.10.10.0/24"
        network.DisplayName = "default"
        network.SetDefaults()
        network.SetNodesLastModified()
        network.SetNetworkLastModified()
        network.KeyUpdateTimeStamp = time.Now().Unix()
        priv := false
        network.IsLocal = &priv
        network.KeyUpdateTimeStamp = time.Now().Unix()
        allow := true
        network.AllowManualSignUp = &allow

        fmt.Println("Creating default network.")

        collection := mongoconn.Client.Database("netmaker").Collection("networks")
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        // insert our network into the network table
        _, err = collection.InsertOne(ctx, network)
        defer cancel()

        }
        if err == nil {
                iscreated = true
        }
        return iscreated, err


}

func GetHTTPPort() (string, error) {

}

func GetGRPCPort() (string, error) {

}

func GetServerDomain() (string, error) {

}

func modConfig(node *nodepb.Node) error{
        network := node.Nodenetwork
        if network == "" {
                return errors.New("No Network Provided")
        }
        modconfig, err := config.ReadConfig(network)
        if err != nil {
                return err
        }
        nodecfg := modconfig.Node
        if node.Name != ""{
                nodecfg.Name = node.Name
        }
        if node.Interface != ""{
                nodecfg.Interface = node.Interface
        }
        if node.Nodenetwork != ""{
                nodecfg.Network = node.Nodenetwork
        }
        if node.Macaddress != ""{
                nodecfg.MacAddress = node.Macaddress
        }
        if node.Localaddress != ""{
                nodecfg.LocalAddress = node.Localaddress
        }
        if node.Postup != ""{
                nodecfg.PostUp = node.Postup
        }
        if node.Postdown != ""{
                nodecfg.PostDown = node.Postdown
        }
        if node.Listenport != 0{
                nodecfg.Port = node.Listenport
        }
        if node.Keepalive != 0{
                nodecfg.KeepAlive = node.Keepalive
        }
        if node.Publickey != ""{
                nodecfg.PublicKey = node.Publickey
        }
        if node.Endpoint != ""{
                nodecfg.Endpoint = node.Endpoint
        }
        if node.Password != ""{
                nodecfg.Password = node.Password
        }
        if node.Address != ""{
                nodecfg.WGAddress = node.Address
        }
        if node.Postchanges != "" {
                nodecfg.PostChanges = node.Postchanges
        }
        if node.Localrange != "" && node.Islocal {
                nodecfg.IsLocal = true
                nodecfg.LocalRange = node.Localrange
        }
        modconfig.Node = nodecfg
        err = config.Write(modconfig, network)
        return err
}

func SetGlobalConfig(globalconf models.GlobalConfig) (error) {

        if err != nil && err != mongo.ErrNoDocuments{
              log.Fatalf("Unable to set global config: %v", err)
        }

        collection := mongoconn.Client.Database("netmaker").Collection("config")
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

        create, _, err := functions.GetGlobalConfig()
        if create {
                _, err := collection.InsertOne(ctx, globalconf)
                defer cancel()
                if err != nil {
                        if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "no documents in result"){
                                return nil
                        } else {
                                return err
                        }
                }
        } else {
                filter := bson.M{"name": "netmaker"}
                update := bson.D{
                        {"$set", bson.D{
                                {"servergrpc", globalconf.ServerGRPC},
                                {"portgrpc", globalconf.PortGRPC},
                        }},
                }
                err := collection.FindOneAndUpdate(ctx, filter, update).Decode(&globalconf)
                        if err == mongo.ErrNoDocuments {
                        //if err == mongo.ErrNoDocuments || strings.Contains(err.Error(), "no documents in result"){
                                return nil
                        }
        }
        return err
}

func DownloadNetclient() error {
	/*
	// Get the data
	resp, err := http.Get("https://github.com/gravitl/netmaker/releases/download/latest/netclient")
	if err != nil {
                fmt.Println("could not download netclient")
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create("/etc/netclient/netclient")
        */
        if !FileExists("/etc/netclient/netclient") {
		_, err := copy("./netclient/netclient", "/etc/netclient/netclient")
	if err != nil {
                fmt.Println("could not create /etc/netclient")
		return err
	}
	}
	//defer out.Close()

	// Write the body to file
	//_, err = io.Copy(out, resp.Body)
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
                return 0, fmt.Errorf("%s is not a regular file", src)
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
                fmt.Println(err)
        }
        return nBytes, err
}

func RemoveNetwork(network string) (bool, error) {
	_, err := os.Stat("/etc/netclient/netclient")
        if err != nil {
                fmt.Println("could not find /etc/netclient")
		return false, err
	}
        cmdoutput, err := exec.Command("/etc/netclient/netclient","-c","remove","-n",network).Output()
        if err != nil {
                fmt.Println(string(cmdoutput))
                return false, err
        }
        fmt.Println("Server removed from network " + network)
        return true, err

}

func AddNetwork(network string) (bool, error) {
	pubip, err := GetPublicIP()
        if err != nil {
                fmt.Println("could not get public IP.")
                return false, err
        }

	_, err = os.Stat("/etc/netclient")
        if os.IsNotExist(err) {
                os.Mkdir("/etc/netclient", 744)
        } else if err != nil {
                fmt.Println("could not find or create /etc/netclient")
                return false, err
        }
	fmt.Println("Directory is ready.")
	token, err := functions.CreateServerToken(network)
        if err != nil {
                fmt.Println("could not create server token for " + network)
		return false, err
        }
	fmt.Println("Token is ready.")
        _, err = os.Stat("/etc/netclient/netclient")
	if os.IsNotExist(err) {
		err = DownloadNetclient()
                fmt.Println("could not download netclient")
		if err != nil {
			return false, err
		}
	}
        err = os.Chmod("/etc/netclient/netclient", 0755)
        if err != nil {
                fmt.Println("could not change netclient directory permissions")
                return false, err
        }
	fmt.Println("Client is ready. Running install.")
	out, err := exec.Command("/etc/netclient/netclient","-c","install","-t",token,"-name","netmaker","-ip4",pubip).Output()
        fmt.Println(string(out))
	if err != nil {
                return false, errors.New(string(out) + err.Error())
        }
	fmt.Println("Server added to network " + network)
	return true, err
}

