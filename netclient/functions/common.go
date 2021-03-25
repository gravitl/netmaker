package functions

import (
	//"github.com/davecgh/go-spew/spew"
	"fmt"
	"time"
	"context"
        "net/http"
        "io/ioutil"
	"io"
	"strings"
	"log"
	"net"
	"os"
	"strconv"
	"os/exec"
        "github.com/gravitl/netmaker/netclient/config"
        nodepb "github.com/gravitl/netmaker/grpc"
	"golang.zx2c4.com/wireguard/wgctrl"
        "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	homedir "github.com/mitchellh/go-homedir"
)

var (
        wcclient nodepb.NodeServiceClient
)

func Install(accesskey string, password string, server string, group string, noauto bool) error {

        wgclient, err := wgctrl.New()

        if err != nil {
                log.Fatalf("failed to open client: %v", err)
        }
        defer wgclient.Close()

	nodecfg := config.Config.Node
	servercfg := config.Config.Server
	fmt.Println("SERVER SETTINGS:")

	if server == "" {
		if servercfg.Address == "" {
			log.Fatal("no server provided")
		} else {
                        server = servercfg.Address
		}
	}
       fmt.Println("     Server: " + server)

	if accesskey == "" {
		if servercfg.AccessKey == "" {
			fmt.Println("no access key provided.Proceeding anyway.")
		} else {
			accesskey = servercfg.AccessKey
		}
	}
       fmt.Println("     AccessKey: " + accesskey)
       err = config.WriteServer(server, accesskey)
        if err != nil {
		fmt.Println("Error encountered while writing Server Config.")
                return err
        }


	fmt.Println("NODE REQUESTING SETTINGS:")
	if password == "" {
		if nodecfg.Password == "" {
			//create error here                
			log.Fatal("no password provided")
		} else {
                        password = nodecfg.Password
                }
	}
       fmt.Println("     Password: " + password)

        if group == "badgroup" {
                if nodecfg.Group == "" {
                        //create error here                
                        log.Fatal("no group provided")
                } else {
			group = nodecfg.Group
		}
        }
       fmt.Println("     Group: " + group)

	var macaddress string
	var localaddress string
	var listenport int32
	var keepalive int32
	var publickey wgtypes.Key
	var privatekey wgtypes.Key
	var privkeystring string
	var endpoint string
	var name string
	var wginterface string

	if nodecfg.Endpoint == "" {
                resp, err := http.Get("https://ifconfig.me")
                if err != nil {
                        return err
                }
       defer resp.Body.Close()
                if resp.StatusCode == http.StatusOK {
                        bodyBytes, err := ioutil.ReadAll(resp.Body)
                if err != nil {
                        return err
                }
                endpoint = string(bodyBytes)
		}
        } else {
		endpoint = nodecfg.Endpoint
	}
       fmt.Println("     Public Endpoint: " + endpoint)

	if nodecfg.LocalAddress == "" {
		ifaces, err := net.Interfaces()
                if err != nil {
                        return err
                }
		var local string
		found := false
		for _, i := range ifaces {
			if i.Flags&net.FlagUp == 0 {
				continue // interface down
			}
			if i.Flags&net.FlagLoopback != 0 {
				continue // loopback interface
			}
			addrs, err := i.Addrs()
			if err != nil {
				return err
			}
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					if !found {
						ip = v.IP
						local = ip.String()
						found = true
					}
				case *net.IPAddr:
					if  !found {
						ip = v.IP
						local = ip.String()
						found = true
					}
				}
			}
		}
		localaddress = local
	} else {
		localaddress = nodecfg.LocalAddress
	}
       fmt.Println("     Local Address: " + localaddress)

        if nodecfg.Name != "" {
                name = nodecfg.Name
        }
       fmt.Println("     Name: " + name)


        if nodecfg.Interface != "" {
                wginterface = nodecfg.Interface
        }
       fmt.Println("     Interface: " + wginterface)

       if nodecfg.KeepAlive != 0 {
                keepalive = nodecfg.KeepAlive
        }
       fmt.Println("     KeepAlive: " + wginterface)


	if nodecfg.Port != 0 {
		listenport = nodecfg.Port
	}
       fmt.Println("     Port: " + string(listenport))

	if nodecfg.PrivateKey != "" {
		privkeystring = nodecfg.PrivateKey
		privatekey, err := wgtypes.ParseKey(nodecfg.PrivateKey)
                if err != nil {
                        log.Fatal(err)
                }
	        if nodecfg.PublicKey != "" {
			publickey, err = wgtypes.ParseKey(nodecfg.PublicKey)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			publickey = privatekey.PublicKey()
		}
	} else {
		privatekey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			log.Fatal(err)
		}
		privkeystring = privatekey.String()
		publickey = privatekey.PublicKey()
	}

	if nodecfg.MacAddress != "" {
		macaddress = nodecfg.MacAddress
	} else {
		macs, err := getMacAddr()
		if err != nil {
			return err
		} else if len(macs) == 0 {
			log.Fatal()
		} else {
			macaddress  = macs[0]
		}
	}
       fmt.Println("     Mac Address: " + macaddress)
       fmt.Println("     Private Key: " + privatekey.String())
       fmt.Println("     Public Key: " + publickey.String())


	var wcclient nodepb.NodeServiceClient
	var requestOpts grpc.DialOption
        requestOpts = grpc.WithInsecure()
        conn, err := grpc.Dial(server, requestOpts)
        if err != nil {
                log.Fatalf("Unable to establish client connection to localhost:50051: %v", err)
        }
        wcclient = nodepb.NewNodeServiceClient(conn)

        postnode := &nodepb.Node{
                Password: password,
                Macaddress: macaddress,
                Accesskey: accesskey,
                Nodegroup:  group,
                Listenport: listenport,
                Keepalive: keepalive,
		Localaddress: localaddress,
		Interface: wginterface,
                Publickey: publickey.String(),
                Name: name,
                Endpoint: endpoint,
        }

       fmt.Println("Writing node settings to wcconfig file.")
        err = modConfig(postnode)
        if err != nil {
                return err
        }

        res, err := wcclient.CreateNode(
                context.TODO(),
                &nodepb.CreateNodeReq{
                        Node: postnode,
                },
        )
        if err != nil {
                return err
        }
        node := res.Node
	fmt.Println("Setting local config from server response")
        if err != nil {
                return err
        }

       fmt.Println("NODE RECIEVED SETTINGS: ")
       fmt.Println("     Password: " + node.Password)
       fmt.Println("     WG Address: " + node.Address)
       fmt.Println("     Group: " + node.Nodegroup)
       fmt.Println("     Public  Endpoint: " + node.Endpoint)
       fmt.Println("     Local Address: " + node.Localaddress)
       fmt.Println("     Name: " + node.Name)
       fmt.Println("     Interface: " + node.Interface)
       fmt.Println("     Port: " + strconv.FormatInt(int64(node.Listenport), 10))
       fmt.Println("     KeepAlive: " + strconv.FormatInt(int64(node.Keepalive), 10))
       fmt.Println("     Public Key: " + node.Publickey)
       fmt.Println("     Mac Address: " + node.Macaddress)

        err = modConfig(node)
        if err != nil {
                return err
        }

	if node.Ispending {
		fmt.Println("Node is marked as PENDING.")
		fmt.Println("Awaiting approval from Admin before configuring WireGuard.")
	        if !noauto {
			fmt.Println("Configuring WireCat Service.")
			err = ConfigureSystemD()
			return err
		}

	}

	peers, err := getPeers(node.Macaddress, node.Nodegroup, server)

	if err != nil {
                return err
        }
	fmt.Println("retrived peers, setting wireguard config.")
	err = storePrivKey(privkeystring)
        if err != nil {
                return err
        }
	err = initWireguard(node, privkeystring, peers)
        if err != nil {
                return err
        }
	if !noauto {
		err = ConfigureSystemD()
	}
        if err != nil {
                return err
        }

	return err
}
func modConfig(node *nodepb.Node) error{
	modconfig := config.Config
	modconfig.ReadConfig()
	nodecfg := modconfig.Node
	if node.Name != ""{
		nodecfg.Name = node.Name
	}
        if node.Interface != ""{
                nodecfg.Interface = node.Interface
        }
        if node.Nodegroup != ""{
                nodecfg.Group = node.Nodegroup
        }
        if node.Macaddress != ""{
                nodecfg.MacAddress = node.Macaddress
        }
        if node.Localaddress != ""{
		nodecfg.LocalAddress = node.Localaddress
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
	modconfig.Node = nodecfg
	err := config.Write(modconfig)
	return err
}


func getMacAddr() ([]string, error) {
    ifas, err := net.Interfaces()
    if err != nil {
        return nil, err
    }
    var as []string
    for _, ifa := range ifas {
        a := ifa.HardwareAddr.String()
        if a != "" {
            as = append(as, a)
        }
    }
    return as, nil
}
/*
func read(macaddress string,  group string) error {
	//this would be  used  for retrieving state as set by the server.
}

func checkLocalConfigChange() error {

}
*/

func initWireguard(node *nodepb.Node, privkey string, peers []wgtypes.PeerConfig) error  {

	ipExec, err := exec.LookPath("ip")
	if err !=  nil {
		return err
	}
	key, err := wgtypes.ParseKey(privkey)
        if err !=  nil {
                return err
        }

        wgclient, err := wgctrl.New()
	modcfg := config.Config
	modcfg.ReadConfig()
	nodecfg := modcfg.Node
	fmt.Println("beginning local WG config")


        if err != nil {
                log.Fatalf("failed to open client: %v", err)
        }
        defer wgclient.Close()

        fmt.Println("setting local settings")

	ifacename := node.Interface
	if nodecfg.Interface != "" {
		ifacename = nodecfg.Interface
	} else if node.Interface != "" {
		ifacename = node.Interface
	} else  {
		log.Fatal("no interface to configure")
	}
	if node.Address == "" {
		log.Fatal("no address to configure")
	}

        cmdIPDevLinkAdd := &exec.Cmd {
                Path: ipExec,
                Args: []string{ ipExec, "link", "add", "dev", ifacename, "type",  "wireguard" },
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        cmdIPAddrAdd := &exec.Cmd {
                Path: ipExec,
                Args: []string{ ipExec, "address", "add", "dev", ifacename, node.Address+"/24"},
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
	err = cmdIPDevLinkAdd.Run()
	if  err  !=  nil && !strings.Contains(err.Error(), "exists") {
		fmt.Println("Error creating interface")
		//fmt.Println(err.Error())
		//return err
	}
        err = cmdIPAddrAdd.Run()
        if  err  !=  nil {
		fmt.Println("Error adding address")
                //return err
        }

	var nodeport int
	nodeport = int(node.Listenport)

	fmt.Println("setting WG config from node and peers")

	//pubkey := privkey.PublicKey()
	conf := wgtypes.Config{
		PrivateKey: &key,
		ListenPort: &nodeport,
		ReplacePeers: true,
		Peers: peers,
	}
	_, err = wgclient.Device(ifacename)
        if err != nil {
                if os.IsNotExist(err) {
                        fmt.Println("Device does not exist: ")
                        fmt.Println(err)
                } else {
                        log.Fatalf("Unknown config error: %v", err)
                }
        }

	fmt.Println("configuring WG device")

	err = wgclient.ConfigureDevice(ifacename, conf)

	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Device does not exist: ")
			fmt.Println(err)
		} else {
			log.Fatalf("Unknown config error: %v", err)
		}
	}
        cmdIPLinkUp := &exec.Cmd {
                Path: ipExec,
                Args: []string{ ipExec, "link", "set", "up", "dev", ifacename},
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        cmdIPLinkDown := &exec.Cmd {
                Path: ipExec,
                Args: []string{ ipExec, "link", "set", "down", "dev", ifacename},
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        err = cmdIPLinkDown.Run()
        err = cmdIPLinkUp.Run()
        if  err  !=  nil {
                return err
        }
	return err
}
/*
func reconfigureWireguardSelf(node  nodepb.Node) error {

}

func reconfigureWireguardPeers(peers []nodepb.PeersResponse) error {

}


func update(node nodepb.Node) error {

}

func updateLocal() error {

}
*/

func setWGConfig() error {
        servercfg := config.Config.Server
        nodecfg := config.Config.Node
        node := getNode()

	peers, err := getPeers(node.Macaddress, nodecfg.Group, servercfg.Address)
        if err != nil {
                return err
        }
	privkey, err := retrievePrivKey()
        if err != nil {
                return err
        }

        err = initWireguard(&node, privkey, peers)
        if err != nil {
                return err
        }

	return err
}

func storePrivKey(key string) error{
	d1 := []byte(key)
	err := ioutil.WriteFile("/root/.wckey", d1, 0644)
	return err
}

func retrievePrivKey() (string, error) {
	dat, err := ioutil.ReadFile("/root/.wckey")
	return string(dat), err
}


func CheckIn() error {
	node := getNode()
        nodecfg := config.Config.Node
	servercfg := config.Config.Server

        var wcclient nodepb.NodeServiceClient
        var requestOpts grpc.DialOption
        requestOpts = grpc.WithInsecure()
        conn, err := grpc.Dial(servercfg.Address, requestOpts)
        if err != nil {
		return err
                log.Fatalf("Unable to establish client connection to localhost:50051: %v", err)
        }
        wcclient = nodepb.NewNodeServiceClient(conn)

        ctx := context.Background()
        fmt.Println("Authenticating with GRPC Server")
        ctx, err = SetJWT(wcclient)
        if err != nil {
		return err
		log.Fatalf("Failed to authenticate: %v", err)
	}
        fmt.Println("Authenticated")

        var header metadata.MD

        checkinres, err := wcclient.CheckIn(
                ctx,
                &nodepb.CheckInReq{
                        Node: &node,
                },
		grpc.Header(&header),
        )
        if err != nil {
		return err
                log.Fatalf("Unable to process Check In request: %v", err)
        }
	fmt.Println("Checked in.")
	/*
	if nodecfg.PostChanges && checkinres.Checkinresponse.Nodeupdated {
		nodecfg.PostChanges = false
		modConfig(readres, false)
	}
	*/
	if  checkinres.Checkinresponse.Ispending {
		fmt.Println("Node is in pending status. Waiting for Admin approval of  node before making furtherupdates.")
		return err
	}
	if checkinres.Checkinresponse.Needconfigupdate {
		fmt.Println("Server has requested that node update config.")
		fmt.Println("Updating config from remote server.")
                req := &nodepb.ReadNodeReq{
                        Macaddress: node.Macaddress,
                        Group: node.Nodegroup,
                }
                readres, err := wcclient.ReadNode(ctx, req, grpc.Header(&header))
                if err != nil {
			return err
                        log.Fatalf("Error: %v", err)
                }
                err = modConfig(readres.Node)
                if err != nil {
			return err
                        log.Fatalf("Error: %v", err)
                }
                err = setWGConfig()
                if err != nil {
			return err
                        log.Fatalf("Error: %v", err)
                }
	} else if nodecfg.PostChanges == "true" {
                fmt.Println("Node has requested to update remote config.")
                fmt.Println("Posting local config to remote server.")
		postnode := getNode()
		req := &nodepb.UpdateNodeReq{
                               Node: &postnode,
                        }
                res, err := wcclient.UpdateNode(ctx, req, grpc.Header(&header))
                if err != nil {
			return err
			log.Fatalf("Error: %v", err)
                }
		res.Node.Postchanges = "false"
		err = modConfig(res.Node)
                if err != nil {
			return err
                        log.Fatalf("Error: %v", err)
                }
		err = setWGConfig()
                if err != nil {
			return err
                        log.Fatalf("Error: %v", err)
                }
	}
        if checkinres.Checkinresponse.Needpeerupdate {
                fmt.Println("Server has requested that node update peer list.")
                fmt.Println("Updating peer list from remote server.")
                err = setWGConfig()
                if err != nil {
			return err
                        log.Fatalf("Unable to process Set Peers request: %v", err)
                }
        }
	return nil
}
func getNode() nodepb.Node {
	modcfg := config.Config
	modcfg.ReadConfig()
	nodecfg := modcfg.Node
	var node nodepb.Node

	node.Name = nodecfg.Name
	node.Interface = nodecfg.Interface
	node.Nodegroup = nodecfg.Group
	node.Localaddress = nodecfg.LocalAddress
	node.Address = nodecfg.WGAddress
	node.Listenport = nodecfg.Port
	node.Keepalive = nodecfg.KeepAlive
	node.Postup = nodecfg.PostUp
	node.Preup = nodecfg.PreUp
	node.Publickey = nodecfg.PublicKey
	node.Macaddress = nodecfg.MacAddress
	node.Endpoint = nodecfg.Endpoint
	node.Password = nodecfg.Password

	//spew.Dump(node)

        return node
}



func Remove() error {
        //need to  implement checkin on server side
        servercfg := config.Config.Server
        node := config.Config.Node
	fmt.Println("Deleting remote node with MAC: " + node.MacAddress)


        var wcclient nodepb.NodeServiceClient
        var requestOpts grpc.DialOption
        requestOpts = grpc.WithInsecure()
        conn, err := grpc.Dial(servercfg.Address, requestOpts)
        if err != nil {
                return err
                log.Fatalf("Unable to establish client connection to localhost:50051: %v", err)
        }
        wcclient = nodepb.NewNodeServiceClient(conn)

        ctx := context.Background()
        fmt.Println("Authenticating with GRPC Server")
        ctx, err = SetJWT(wcclient)
        if err != nil {
                return err
                log.Fatalf("Failed to authenticate: %v", err)
        }
        fmt.Println("Authenticated")

        var header metadata.MD

        _, err = wcclient.DeleteNode(
                ctx,
                &nodepb.DeleteNodeReq{
                        Macaddress: node.MacAddress,
                        GroupName: node.Group,
                },
                grpc.Header(&header),
        )
        if err != nil {
		fmt.Println("Encountered error deleting node.")
		fmt.Println(err)
                //return err
                //log.Fatalf("Unable to process Delete request: %v", err)
        }
        fmt.Println("Deleted node " + node.MacAddress)

	err = WipeLocal()
	if err != nil {
                return err
                log.Fatalf("Unable to wipe local config: %v", err)
	}
	err =  RemoveSystemDServices()
        if err != nil {
                return err
                log.Fatalf("Unable to remove systemd services: %v", err)
        }

	return nil
}

func WipeLocal() error{
        nodecfg := config.Config.Node
        ifacename := nodecfg.Interface

        home, err := homedir.Dir()
        if err != nil {
                log.Fatal(err)
        }
        err = os.Remove(home + "/.wcconfig")
        if  err  !=  nil {
                fmt.Println(err)
        }
        err = os.Remove(home + "/.wctoken")
        if  err  !=  nil {
                fmt.Println(err)
        }
        ipExec, err := exec.LookPath("ip")

        cmdIPLinkDel := &exec.Cmd {
                Path: ipExec,
                Args: []string{ ipExec, "link", "del", ifacename },
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        err = cmdIPLinkDel.Run()
        if  err  !=  nil {
                fmt.Println(err)
        }
	return err

}


func getPeers(macaddress string, group string, server string) ([]wgtypes.PeerConfig, error) {
        //need to  implement checkin on server side
	var peers []wgtypes.PeerConfig
	var wcclient nodepb.NodeServiceClient
        modcfg := config.Config
        modcfg.ReadConfig()
        nodecfg := modcfg.Node
	keepalive := nodecfg.KeepAlive
	keepalivedur, err := time.ParseDuration(strconv.FormatInt(int64(keepalive), 10) + "s")
        if err != nil {
                log.Fatalf("Issue with format of keepalive value. Please update wcconfig: %v", err)
        }


	fmt.Println("Registering with GRPC Server")
	requestOpts := grpc.WithInsecure()
	conn, err := grpc.Dial(server, requestOpts)
	if err != nil {
		log.Fatalf("Unable to establish client connection to localhost:50051: %v", err)
	}
	// Instantiate the BlogServiceClient with our client connection to the server
	wcclient = nodepb.NewNodeServiceClient(conn)

	req := &nodepb.GetPeersReq{
		Macaddress: macaddress,
                Group: group,
        }
        ctx := context.Background()
	fmt.Println("Authenticating with GRPC Server")
	ctx, err = SetJWT(wcclient)
        if err != nil {
		fmt.Println("Failed to authenticate.")
                return peers, err
        }
        var header metadata.MD

        stream, err := wcclient.GetPeers(ctx, req, grpc.Header(&header))
	if err != nil {
                return nil, err
        }
	fmt.Println("Parsing  peers response")
        for {
                // stream.Recv returns a pointer to a ListBlogRes at the current iteration
		res, err := stream.Recv()
                // If end of stream, break the loop

		if err == io.EOF {
                        break
                }
                // if err, return an error
                if err != nil {
			if strings.Contains(err.Error(), "mongo: no documents in result") {
				break
			} else {
			fmt.Println("ERROR ENCOUNTERED WITH RESPONSE")
			fmt.Println(res)
                        return peers, err
			}
                }
		pubkey, err := wgtypes.ParseKey(res.Peers.Publickey)
                if err != nil {
			fmt.Println("error parsing key")
                        return peers, err
                }
		var peer wgtypes.PeerConfig
		if keepalive != 0 {
		peer = wgtypes.PeerConfig{
			PublicKey: pubkey,
			PersistentKeepaliveInterval: &keepalivedur,
			Endpoint: &net.UDPAddr{
				IP:   net.ParseIP(res.Peers.Endpoint),
				Port: int(res.Peers.Listenport),
			},
			ReplaceAllowedIPs: true,
                        AllowedIPs: []net.IPNet{{
                                IP: net.ParseIP(res.Peers.Address),
				Mask: net.CIDRMask(32, 32),
			}},
		}
		} else {
                peer = wgtypes.PeerConfig{
                        PublicKey: pubkey,
                        Endpoint: &net.UDPAddr{
                                IP:   net.ParseIP(res.Peers.Endpoint),
                                Port: int(res.Peers.Listenport),
                        },
                        ReplaceAllowedIPs: true,
                        AllowedIPs: []net.IPNet{{
                                IP: net.ParseIP(res.Peers.Address),
                                Mask: net.CIDRMask(32, 32),
                        }},
                }
		}
		peers = append(peers, peer)

        }
	fmt.Println("Finished parsing peers response")
	return peers, err
}
