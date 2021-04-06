package functions

import (
	//"github.com/davecgh/go-spew/spew"
	"fmt"
	"time"
	"errors"
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
	"encoding/base64"
	"google.golang.org/grpc/metadata"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	//homedir "github.com/mitchellh/go-homedir"
)

var (
        wcclient nodepb.NodeServiceClient
)

func Install(accesskey string, password string, server string, group string, noauto bool, accesstoken string) error {


	tserver := ""
	tnetwork := ""
	tkey := ""

	if accesstoken != "" && accesstoken != "badtoken" {
		btoken, err := base64.StdEncoding.DecodeString(accesstoken)
		if err  != nil {
			log.Fatalf("Something went wrong decoding your token: %v", err)
		}
		token := string(btoken)
		tokenvals := strings.Split(token, ".")
		tserver = tokenvals[0]
		tnetwork = tokenvals[1]
		tkey = tokenvals[2]
		server = tserver
		group = tnetwork
		accesskey = tkey
		fmt.Println("Decoded values from token:")
		fmt.Println("    Server: " + tserver)
		fmt.Println("    Network: " + tnetwork)
		fmt.Println("    Key: " + tkey)
	}
        wgclient, err := wgctrl.New()

        if err != nil {
                log.Fatalf("failed to open client: %v", err)
        }
        defer wgclient.Close()

	cfg, err := config.ReadConfig(group)
        if err != nil {
                log.Printf("No Config Yet. Will Write: %v", err)
        }
	nodecfg := cfg.Node
	servercfg := cfg.Server
	fmt.Println("SERVER SETTINGS:")

	if server == "" {
		if servercfg.Address == "" && tserver == "" {
			log.Fatal("no server provided")
		} else {
                        server = servercfg.Address
		}
	}
	if tserver != "" {
		server = tserver
	}
       fmt.Println("     Server: " + server)

	if accesskey == "" {
		if servercfg.AccessKey == "" && tkey == "" {
			fmt.Println("no access key provided.Proceeding anyway.")
		} else {
			accesskey = servercfg.AccessKey
		}
	}
	if tkey != "" {
		accesskey = tkey
	}
       fmt.Println("     AccessKey: " + accesskey)
       err = config.WriteServer(server, accesskey, group)
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
                if nodecfg.Group == "" && tnetwork == "" {
                        //create error here                
                        log.Fatal("no group provided")
                } else {
			group = nodecfg.Group
		}
        }
	if tnetwork != "" {
		group =  tnetwork
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
		endpoint, err = getPublicIP()
                if err != nil {
                        return err
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

       fmt.Println("Writing node settings to netconfig file.")
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
			fmt.Println("Configuring Netmaker Service.")
			err = ConfigureSystemD(group)
			return err
		}

	}

	peers, err := getPeers(node.Macaddress, group, server)

	if err != nil {
                return err
        }
	fmt.Println("retrived peers, setting wireguard config.")
	err = storePrivKey(privkeystring, group)
        if err != nil {
                return err
        }
	err = initWireguard(node, privkeystring, peers)
        if err != nil {
                return err
        }
	if !noauto {
		err = ConfigureSystemD(group)
	}
        if err != nil {
                return err
        }

	return err
}

func getPublicIP() (string, error) {

	iplist := []string{"https://ifconfig.me", "http://api.ipify.org", "http://ipinfo.io/ip"}
	endpoint := ""
	var err error
	    for _, ipserver := range iplist {
		resp, err := http.Get(ipserver)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			bodyBytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				continue
			}
			endpoint = string(bodyBytes)
			break
		}

	}
	if err == nil && endpoint == "" {
		err =  errors.New("Public Address Not Found.")
	}
	return endpoint, err
}

func modConfig(node *nodepb.Node) error{
	group := node.Nodegroup
	if group == "" {
		return errors.New("No Group Provided")
	}
	//modconfig := config.Config
	modconfig, err := config.ReadConfig(group)
	//modconfig.ReadConfig()
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
	err = config.Write(modconfig, group)
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
	//modcfg := config.Config
	//modcfg.ReadConfig()
	modcfg, err := config.ReadConfig(node.Nodegroup)
        if err != nil {
                return err
        }


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


         currentiface, err := net.InterfaceByName(ifacename)


        if err != nil {
		err = cmdIPDevLinkAdd.Run()
	if  err  !=  nil && !strings.Contains(err.Error(), "exists") {
		fmt.Println("Error creating interface")
		//fmt.Println(err.Error())
		//return err
	}
	}
	match := false
	addrs, _ := currentiface.Addrs()
	for _, a := range addrs {
		if strings.Contains(a.String(), node.Address){
			match = true
		}
	}
	if !match {
        err = cmdIPAddrAdd.Run()
        if  err  !=  nil {
		fmt.Println("Error adding address")
                //return err
        }
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
			fmt.Printf("This is inconvenient: %v", err)
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

func setWGKeyConfig(network string, serveraddr string) error {

        ctx := context.Background()
        var header metadata.MD

        var wcclient nodepb.NodeServiceClient
        var requestOpts grpc.DialOption
        requestOpts = grpc.WithInsecure()
        conn, err := grpc.Dial(serveraddr, requestOpts)
        if err != nil {
                fmt.Printf("Cant dial GRPC server: %v", err)
                return err
        }
        wcclient = nodepb.NewNodeServiceClient(conn)

	fmt.Println("Authenticating with GRPC Server")
        ctx, err = SetJWT(wcclient, network)
        if err != nil {
                fmt.Printf("Failed to authenticate: %v", err)
                return err
        }
        fmt.Println("Authenticated")


	node := getNode(network)

	privatekey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return err
	}
	privkeystring := privatekey.String()
	publickey := privatekey.PublicKey()

	node.Publickey = publickey.String()

	err = storePrivKey(privkeystring, network)
        if err != nil {
                return err
        }
        err = modConfig(&node)
        if err != nil {
                return err
        }


	postnode := getNode(network)

        req := &nodepb.UpdateNodeReq{
               Node: &postnode,
        }

        _, err = wcclient.UpdateNode(ctx, req, grpc.Header(&header))
        if err != nil {
                return err
        }
        err = setWGConfig(network)
        if err != nil {
                return err
                log.Fatalf("Error: %v", err)
        }

        return err
}


func setWGConfig(network string) error {

        cfg, err := config.ReadConfig(network)
        if err != nil {
                return err
        }
	servercfg := cfg.Server
        nodecfg := cfg.Node
        node := getNode(network)

	peers, err := getPeers(node.Macaddress, nodecfg.Group, servercfg.Address)
        if err != nil {
                return err
        }
	privkey, err := retrievePrivKey(network)
        if err != nil {
                return err
        }

        err = initWireguard(&node, privkey, peers)
        if err != nil {
                return err
        }

	return err
}

func storePrivKey(key string, network string) error{
	d1 := []byte(key)
	err := ioutil.WriteFile("/etc/netclient/wgkey-" + network, d1, 0644)
	return err
}

func retrievePrivKey(network string) (string, error) {
	dat, err := ioutil.ReadFile("/etc/netclient/wgkey-" + network)
	return string(dat), err
}

func getPrivateAddr() (string, error) {
                ifaces, err := net.Interfaces()
                if err != nil {
                        return "", err
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
                                return "", err
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
		if !found {
			err := errors.New("Local Address Not Found.")
			return "", err
		}
		return local, err
}


func CheckIn(network string) error {
	node := getNode(network)
        cfg, err := config.ReadConfig(network)
        if err != nil {
                return err
        }
	nodecfg := cfg.Node
	servercfg := cfg.Server
	fmt.Println("Checking into server: " + servercfg.Address)

	setupcheck := true
	ipchange := false

	if !nodecfg.RoamingOff {
		fmt.Println("Checking to see if addresses have changed")
		extIP, err := getPublicIP()
		if err != nil {
			fmt.Printf("Error encountered checking ip addresses: %v", err)
		}
		if nodecfg.Endpoint != extIP  && extIP != "" {
	                fmt.Println("Endpoint has changed from " +
			nodecfg.Endpoint + " to " + extIP)
			fmt.Println("Updating address")
			nodecfg.Endpoint = extIP
			nodecfg.PostChanges = "true"
			node.Endpoint = extIP
			node.Postchanges = "true"
			ipchange = true
		}
		intIP, err := getPrivateAddr()
                if err != nil {
                        fmt.Printf("Error encountered checking ip addresses: %v", err)
                }
                if nodecfg.LocalAddress != intIP  && intIP != "" {
                        fmt.Println("Local Address has changed from " +
			nodecfg.LocalAddress + " to " + intIP)
			fmt.Println("Updating address")
			nodecfg.LocalAddress = intIP
			nodecfg.PostChanges = "true"
			node.Localaddress = intIP
			node.Postchanges = "true"
			ipchange = true
                }
		if node.Postchanges != "true" {
			fmt.Println("Addresses have not changed.")
		}
	}
	if ipchange {
		err := modConfig(&node)
                if err != nil {
                        return err
                        log.Fatalf("Error: %v", err)
                }
                err = setWGConfig(network)
                if err != nil {
                        return err
                        log.Fatalf("Error: %v", err)
                }
	        node = getNode(network)
		cfg, err := config.ReadConfig(network)
		if err != nil {
			return err
		}
		nodecfg = cfg.Node
	}


        var wcclient nodepb.NodeServiceClient
        var requestOpts grpc.DialOption
        requestOpts = grpc.WithInsecure()
        conn, err := grpc.Dial(servercfg.Address, requestOpts)
        if err != nil {
		fmt.Printf("Cant dial GRPC server: %v", err)
		return err
        }
        wcclient = nodepb.NewNodeServiceClient(conn)

        ctx := context.Background()
        fmt.Println("Authenticating with GRPC Server")
        ctx, err = SetJWT(wcclient, network)
        if err != nil {
                fmt.Printf("Failed to authenticate: %v", err)
		return err
	}
        fmt.Println("Authenticated")
        fmt.Println("Checking In.")

        var header metadata.MD

        checkinres, err := wcclient.CheckIn(
                ctx,
                &nodepb.CheckInReq{
                        Node: &node,
                },
		grpc.Header(&header),
        )
        if err != nil {
        if  checkinres != nil && checkinres.Checkinresponse.Ispending {
                fmt.Println("Node is in pending status. Waiting for Admin approval of  node before making further updates.")
                return nil
        }
                fmt.Printf("Unable to process Check In request: %v", err)
		return err
        }
	fmt.Println("Checked in.")
	if  checkinres.Checkinresponse.Ispending {
		fmt.Println("Node is in pending status. Waiting for Admin approval of  node before making further updates.")
		return err
	}

                newinterface := getNode(network).Interface
                readreq := &nodepb.ReadNodeReq{
                        Macaddress: node.Macaddress,
                        Group: node.Nodegroup,
                }
                readres, err := wcclient.ReadNode(ctx, readreq, grpc.Header(&header))
                if err != nil {
                        fmt.Printf("Error: %v", err)
                } else {
                currentiface := readres.Node.Interface
                ifaceupdate := newinterface != currentiface
                if err != nil {
                        log.Printf("Error retrieving interface: %v", err)
                }
                if ifaceupdate {
			fmt.Println("Interface update: " + currentiface +
			" >>>> " + newinterface)
                        err := DeleteInterface(currentiface)
                        if err != nil {
                                fmt.Println("ERROR DELETING INTERFACE: " + currentiface)
                        }
                }
                err = setWGConfig(network)
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
                err = setWGConfig(network)
                if err != nil {
			return err
                        log.Fatalf("Error: %v", err)
                }
		setupcheck = false
	} else if nodecfg.PostChanges == "true" {
                fmt.Println("Node has requested to update remote config.")
                fmt.Println("Posting local config to remote server.")
		postnode := getNode(network)

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
		err = setWGConfig(network)
                if err != nil {
			return err
                        log.Fatalf("Error: %v", err)
                }
		setupcheck = false
	}
        if checkinres.Checkinresponse.Needkeyupdate {
                fmt.Println("Server has requested that node update key pairs.")
                fmt.Println("Proceeding to re-generate key pairs for Wiregard.")
                err = setWGKeyConfig(network, servercfg.Address)
                if err != nil {
                        return err
                        log.Fatalf("Unable to process reset keys request: %v", err)
                }
                setupcheck = false
        }
        if checkinres.Checkinresponse.Needpeerupdate {
                fmt.Println("Server has requested that node update peer list.")
                fmt.Println("Updating peer list from remote server.")
                err = setWGConfig(network)
                if err != nil {
			return err
                        log.Fatalf("Unable to process Set Peers request: %v", err)
                }
		setupcheck = false
        }
	if checkinres.Checkinresponse.Needdelete {
		fmt.Println("This machine got the delete signal. Deleting.")
                err := Remove(network)
                if err != nil {
                        return err
                        log.Fatalf("Error: %v", err)
                }
	}
	if setupcheck {
	iface := nodecfg.Interface
	_, err := net.InterfaceByName(iface)
        if err != nil {
		fmt.Println("interface " + iface + " does not currently exist. Setting up WireGuard.")
                err = setWGConfig(network)
                if err != nil {
                        return err
                        log.Fatalf("Error: %v", err)
                }
	}
	}
	return nil
}

func needInterfaceUpdate(ctx context.Context, mac string, group string, iface string) (bool, string, error) {
                var header metadata.MD
		req := &nodepb.ReadNodeReq{
                        Macaddress: mac,
                        Group: group,
                }
                readres, err := wcclient.ReadNode(ctx, req, grpc.Header(&header))
                if err != nil {
                        return false, "", err
                        log.Fatalf("Error: %v", err)
                }
		oldiface := readres.Node.Interface

		return iface != oldiface, oldiface, err
}

func getNode(network string) nodepb.Node {

        modcfg, err := config.ReadConfig(network)
        if err != nil {
                log.Fatalf("Error: %v", err)
        }

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



func Remove(network string) error {
        //need to  implement checkin on server side
        cfg, err := config.ReadConfig(network)
        if err != nil {
                return err
        }
	servercfg := cfg.Server
        node := cfg.Node
	fmt.Println("Deleting remote node with MAC: " + node.MacAddress)


        var wcclient nodepb.NodeServiceClient
        var requestOpts grpc.DialOption
        requestOpts = grpc.WithInsecure()
        conn, err := grpc.Dial(servercfg.Address, requestOpts)
	if err != nil {
                log.Printf("Unable to establish client connection to " + servercfg.Address + ": %v", err)
		//return err
        }else {
        wcclient = nodepb.NewNodeServiceClient(conn)

        ctx := context.Background()
        fmt.Println("Authenticating with GRPC Server")
        ctx, err = SetJWT(wcclient, network)
        if err != nil {
                //return err
                log.Printf("Failed to authenticate: %v", err)
        } else {
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
		log.Printf("Encountered error deleting node: %v", err)
		fmt.Println(err)
        } else {
		fmt.Println("Deleted node " + node.MacAddress)
	}
	}
	}
	err = WipeLocal(network)
	if err != nil {
                log.Printf("Unable to wipe local config: %v", err)
	}
	err =  RemoveSystemDServices(network)
        if err != nil {
                return err
                log.Printf("Unable to remove systemd services: %v", err)
        }
	fmt.Printf("Please investigate any stated errors to ensure proper removal.")
	fmt.Printf("Failure to delete node from server via gRPC will mean node still exists and needs to be manually deleted by administrator.")

	return nil
}

func WipeLocal(network string) error{
        cfg, err := config.ReadConfig(network)
        if err != nil {
                return err
        }
        nodecfg := cfg.Node
        ifacename := nodecfg.Interface

        //home, err := homedir.Dir()
	home := "/etc/netclient"
	err = os.Remove(home + "/netconfig-" + network)
        if  err  !=  nil {
                fmt.Println(err)
        }
        err = os.Remove(home + "/nettoken")
        if  err  !=  nil {
                fmt.Println(err)
        }
        ipExec, err := exec.LookPath("ip")

	if ifacename != "" {
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
	}
	return err

}

func DeleteInterface(ifacename string) error{
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
        cfg, err := config.ReadConfig(group)
        if err != nil {
		log.Fatalf("Issue retrieving config for network: " + group +  ". Please investigate: %v", err)
        }
        nodecfg := cfg.Node
	keepalive := nodecfg.KeepAlive
	keepalivedur, err := time.ParseDuration(strconv.FormatInt(int64(keepalive), 10) + "s")
        if err != nil {
                log.Fatalf("Issue with format of keepalive value. Please update netconfig: %v", err)
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
	ctx, err = SetJWT(wcclient, group)
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
