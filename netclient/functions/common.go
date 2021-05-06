package functions

import (
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

func ListPorts() error{
	wgclient, err := wgctrl.New()
	if err  != nil {
		return err
	}
	devices, err := wgclient.Devices()
        if err  != nil {
                return err
        }
	fmt.Println("Here are your ports:")
	 for _, i := range devices {
		fmt.Println(i.ListenPort)
	}
	return err
}

func GetFreePort(rangestart int32) (int32, error){
        wgclient, err := wgctrl.New()
        if err  != nil {
                return 0, err
        }
        devices, err := wgclient.Devices()
        if err  != nil {
                return 0, err
        }
	var portno int32
	portno = 0
	for  x := rangestart; x <= 60000; x++ {
		conflict := false
		for _, i := range devices {
			if int32(i.ListenPort) == x {
				conflict = true
				break;
			}
		}
		if conflict {
			continue
		}
		portno = x
		break
	}
        return portno, err
}

func Install(accesskey string, password string, server string, network string, noauto bool, accesstoken string,  inputname string, dnsoff bool) error {

	tserver := ""
	tnetwork := ""
	tkey := ""
	trange := ""
	var localrange *net.IPNet
	islocal := false
	if FileExists("/etc/systemd/system/netclient-"+network+".timer") ||
	   FileExists("/etc/netclient/netconfig-"+network) {
		   err := errors.New("ALREADY_INSTALLED. Netclient appears to already be installed for network " + network + ". To re-install, please remove by executing 'sudo netclient -c remove -n " + network + "'. Then re-run the install command.")
		return err
	}

	if accesstoken != "" && accesstoken != "badtoken" {
		btoken, err := base64.StdEncoding.DecodeString(accesstoken)
		if err  != nil {
			log.Fatalf("Something went wrong decoding your token: %v", err)
		}
		token := string(btoken)
		tokenvals := strings.Split(token, "|")
		tserver = tokenvals[0]
		tnetwork = tokenvals[1]
		tkey = tokenvals[2]
		trange = tokenvals[3]
		printrange := ""
		if server == "localhost:50051" {
			server = tserver
		}
		if network == "nonetwork" {
			network = tnetwork
		}
		if accesskey == "badkey" {
			accesskey = tkey
		}
		fmt.Println(trange)
		if trange != "" {
			fmt.Println("This is a local network. Proceeding with local address as endpoint.")
			islocal = true
			_, localrange, err = net.ParseCIDR(trange)
			if err == nil {
				printrange = localrange.String()
			} else {
				//localrange = ""
			}
		} else {
			printrange = "Not a local network. Will use public address for endpoint."
		}

		fmt.Println("Decoded values from token:")
		fmt.Println("    Server: " + server)
		fmt.Println("    Network: " + network)
		fmt.Println("    Key: " + accesskey)
		fmt.Println("    Local Range: " + printrange)
	}

        wgclient, err := wgctrl.New()

        if err != nil {
                log.Fatalf("failed to open client: %v", err)
        }
        defer wgclient.Close()

	cfg, err := config.ReadConfig(network)
        if err != nil {
                log.Printf("No Config Yet. Will Write: %v", err)
        }
	nodecfg := cfg.Node
	servercfg := cfg.Server
	fmt.Println("SERVER SETTINGS:")

	nodecfg.DNSOff = dnsoff

	if server == "" {
		if servercfg.Address == "" && tserver == "" {
			log.Fatal("no server provided")
		} else {
                        server = servercfg.Address
		}
	}
       fmt.Println("     Server: " + server)

	if accesskey == "" {
		if servercfg.AccessKey == "" && tkey == "" {
			fmt.Println("no access key provided.Proceeding anyway.")
		} else {
			accesskey = servercfg.AccessKey
		}
	}
       fmt.Println("     AccessKey: " + accesskey)
       err = config.WriteServer(server, accesskey, network)
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

        if network == "badnetwork" {
                if nodecfg.Network == "" && tnetwork == "" {
                        //create error here                
                        log.Fatal("no network provided")
                } else {
			network = nodecfg.Network
		}
        }
       fmt.Println("     Network: " + network)

	var macaddress string
	var localaddress string
	var listenport int32
	var keepalive int32
	var publickey wgtypes.Key
	var privatekey wgtypes.Key
	var privkeystring string
	var endpoint string
	var postup string
	var postdown string
	var name string
	var wginterface string

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
						if islocal {
							found = localrange.Contains(ip)
						} else {
							found = true
						}
					}
				case *net.IPAddr:
					if  !found {
						ip = v.IP
						local = ip.String()
						if islocal {
							found = localrange.Contains(ip)

						} else {
							found = true
						}
					}
				}
			}
		}
		localaddress = local
	} else {
		localaddress = nodecfg.LocalAddress
	}
       fmt.Println("     Local Address: " + localaddress)

        if nodecfg.Endpoint == "" {
		if islocal && localaddress != "" {
			endpoint = localaddress
			fmt.Println("Endpoint is local. Setting to address: " + endpoint)
		} else {

			endpoint, err = getPublicIP()
			if err != nil {
				fmt.Println("Error setting endpoint.")
				return err
			}
			fmt.Println("Endpoint is public. Setting to address: " + endpoint)
		}
        } else {
                endpoint = nodecfg.Endpoint
		fmt.Println("Endpoint set in config. Setting to address: " + endpoint)
        }
       fmt.Println("     Endpoint: " + endpoint)


        if nodecfg.Name != "" {
                name = nodecfg.Name
        }
	if inputname != "" && inputname != "noname" {
		name = inputname
	}
       fmt.Println("     Name: " + name)


        if nodecfg.Interface != "" {
                wginterface = nodecfg.Interface
        }
       fmt.Println("     Interface: " + wginterface)

        if nodecfg.PostUp != "" {
                postup = nodecfg.PostUp
        }
       fmt.Println("     PostUp: " + postup)

       if nodecfg.PostDown!= "" {
                postdown = nodecfg.PostDown
        }
       fmt.Println("     PostDown: " + postdown)


       if nodecfg.KeepAlive != 0 {
                keepalive = nodecfg.KeepAlive
        }
       fmt.Println("     KeepAlive: " + wginterface)


	if nodecfg.Port != 0 {
		listenport = nodecfg.Port
	}
	if listenport == 0 {
		listenport, err = GetFreePort(51821)
		if err != nil {
			fmt.Printf("Error retrieving port: %v", err)
		}
	}
       fmt.Printf("     Port: %v", listenport)
       fmt.Println("")

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
                Nodenetwork:  network,
                Listenport: listenport,
                Postup: postup,
                Postdown: postdown,
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
       fmt.Println("     Network: " + node.Nodenetwork)
       fmt.Println("     Public  Endpoint: " + node.Endpoint)
       fmt.Println("     Local Address: " + node.Localaddress)
       fmt.Println("     Name: " + node.Name)
       fmt.Println("     Interface: " + node.Interface)
       fmt.Println("     PostUp: " + node.Postup)
       fmt.Println("     PostDown: " + node.Postdown)
       fmt.Println("     Port: " + strconv.FormatInt(int64(node.Listenport), 10))
       fmt.Println("     KeepAlive: " + strconv.FormatInt(int64(node.Keepalive), 10))
       fmt.Println("     Public Key: " + node.Publickey)
       fmt.Println("     Mac Address: " + node.Macaddress)
       fmt.Println("     Is Local?: " + strconv.FormatBool(node.Islocal))
       fmt.Println("     Local Range: " + node.Localrange)

	if !islocal && node.Islocal && node.Localrange != "" {
		fmt.Println("Resetting local settings for local network.")
		node.Localaddress, err = getLocalIP(node.Localrange)
		if err != nil {
			return err
		}
		node.Endpoint = node.Localaddress
	}

        err = modConfig(node)
        if err != nil {
                return err
        }

	if node.Ispending {
		fmt.Println("Node is marked as PENDING.")
		fmt.Println("Awaiting approval from Admin before configuring WireGuard.")
	        if !noauto {
			fmt.Println("Configuring Netmaker Service.")
			err = ConfigureSystemD(network)
			return err
		}
	}

	peers, hasGateway, gateways, err := getPeers(node.Macaddress, network, server)

	if err != nil {
                return err
        }
	fmt.Println("retrived peers, setting wireguard config.")
	err = storePrivKey(privkeystring, network)
        if err != nil {
                return err
        }
	err = initWireguard(node, privkeystring, peers, hasGateway, gateways)
        if err != nil {
                return err
        }
	if !noauto {
		err = ConfigureSystemD(network)
	}
        if err != nil {
                return err
        }

	return err
}

func getLocalIP(localrange string) (string, error) {
	_, localRange, err := net.ParseCIDR(localrange)
        if err != nil {
                return "", err
        }
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
                                 found = localRange.Contains(ip)
                        }
		case *net.IPAddr:
			if !found {
				ip = v.IP
                                local = ip.String()
                                found = localRange.Contains(ip)
                        }
                }
        }
        }
	if !found || local == "" {
		return "", errors.New("Failed to find local IP in range " + localrange)
	}
	return local, nil
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


func initWireguard(node *nodepb.Node, privkey string, peers []wgtypes.PeerConfig, hasGateway bool, gateways []string) error  {

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
	modcfg, err := config.ReadConfig(node.Nodenetwork)
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
        if nodecfg.PostDown != "" {
		runcmds := strings.Split(nodecfg.PostDown, "; ")
		err = runCmds(runcmds)
		if err != nil {
			fmt.Println("Error encountered running PostDown: " + err.Error())
		}
	}

	err = cmdIPLinkUp.Run()
        if  err  !=  nil {
                return err
        }

	if nodecfg.PostUp != "" {
                runcmds := strings.Split(nodecfg.PostUp, "; ")
                err = runCmds(runcmds)
                if err != nil {
                        fmt.Println("Error encountered running PostUp: " + err.Error())
                }
        }
	if (hasGateway) {
		for _, gateway := range gateways {
			out, err := exec.Command(ipExec,"-4","route","add",gateway,"dev",ifacename).Output()
                fmt.Println(string(out))
		if err != nil {
                        fmt.Println("Error encountered adding gateway: " + err.Error())
                }
	}
	}
	return err
}
func runCmds(commands []string) error {
	var err error
	for _, command := range commands {
		fmt.Println("Running command: " + command)
		args := strings.Fields(command)
		out, err := exec.Command(args[0], args[1:]...).Output()
		fmt.Println(string(out))
		if err != nil {
			return err
		}
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

	peers, hasGateway, gateways, err := getPeers(node.Macaddress, nodecfg.Network, servercfg.Address)
        if err != nil {
                return err
        }
	privkey, err := retrievePrivKey(network)
        if err != nil {
                return err
        }

        err = initWireguard(&node, privkey, peers, hasGateway, gateways)
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

	if !nodecfg.DNSOff {
		vals := strings.Split(servercfg.Address, ":")
		server := vals[0]
		err = SetDNS(server)
		if err != nil {
                        fmt.Printf("Error encountered setting dns: %v", err)
		}
	}

	if !nodecfg.RoamingOff {
		if !nodecfg.IsLocal {
		fmt.Println("Checking to see if public addresses have changed")
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
		} else {
                fmt.Println("Checking to see if local addresses have changed")
                localIP, err := getLocalIP(nodecfg.LocalRange)
                if err != nil {
                        fmt.Printf("Error encountered checking ip addresses: %v", err)
                }
                if nodecfg.Endpoint != localIP  && localIP != "" {
                        fmt.Println("Endpoint has changed from " +
                        nodecfg.Endpoint + " to " + localIP)
                        fmt.Println("Updating address")
                        nodecfg.Endpoint = localIP
                        nodecfg.LocalAddress = localIP
                        nodecfg.PostChanges = "true"
                        node.Endpoint = localIP
                        node.Localaddress = localIP
                        node.Postchanges = "true"
                        ipchange = true
                }
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
	node.Nodenetwork = network
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
                        Network: node.Nodenetwork,
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
                        err := DeleteInterface(currentiface, nodecfg.PostDown)
                        if err != nil {
                                fmt.Println("ERROR DELETING INTERFACE: " + currentiface)
                        }
                err = setWGConfig(network)
                if err != nil {
                        log.Printf("Error updating interface: %v", err)
                }
		}
		}

	if checkinres.Checkinresponse.Needconfigupdate {
		fmt.Println("Server has requested that node update config.")
		fmt.Println("Updating config from remote server.")
                req := &nodepb.ReadNodeReq{
                        Macaddress: node.Macaddress,
                        Network: node.Nodenetwork,
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
                err = setWGKeyConfig(network, servercfg.Address)
                if err != nil {
                        return err
                        log.Fatalf("Error: %v", err)
                }
	}
	}
	return nil
}

func needInterfaceUpdate(ctx context.Context, mac string, network string, iface string) (bool, string, error) {
                var header metadata.MD
		req := &nodepb.ReadNodeReq{
                        Macaddress: mac,
                        Network: network,
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
	node.Nodenetwork = nodecfg.Network
	node.Localaddress = nodecfg.LocalAddress
	node.Address = nodecfg.WGAddress
	node.Listenport = nodecfg.Port
	node.Keepalive = nodecfg.KeepAlive
	node.Postup = nodecfg.PostUp
	node.Postdown = nodecfg.PostDown
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
                        NetworkName: node.Network,
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
        err = os.Remove(home + "/nettoken-" + network)
        if  err  !=  nil {
                fmt.Println(err)
        }

        err = os.Remove(home + "/wgkey-" + network)
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
        if nodecfg.PostDown != "" {
                runcmds := strings.Split(nodecfg.PostDown, "; ")
                err = runCmds(runcmds)
                if err != nil {
                        fmt.Println("Error encountered running PostDown: " + err.Error())
                }
        }
	}
	return err

}

func DeleteInterface(ifacename string, postdown string) error{
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
        if postdown != "" {
                runcmds := strings.Split(postdown, "; ")
                err = runCmds(runcmds)
                if err != nil {
                        fmt.Println("Error encountered running PostDown: " + err.Error())
                }
        }
        return err
}

func getPeers(macaddress string, network string, server string) ([]wgtypes.PeerConfig, bool, []string, error) {
        //need to  implement checkin on server side
	hasGateway := false
	var gateways []string
	var peers []wgtypes.PeerConfig
	var wcclient nodepb.NodeServiceClient
        cfg, err := config.ReadConfig(network)
        if err != nil {
		log.Fatalf("Issue retrieving config for network: " + network +  ". Please investigate: %v", err)
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
                Network: network,
        }
        ctx := context.Background()
	fmt.Println("Authenticating with GRPC Server")
	ctx, err = SetJWT(wcclient, network)
        if err != nil {
		fmt.Println("Failed to authenticate.")
                return peers, hasGateway, gateways, err
        }
        var header metadata.MD

        stream, err := wcclient.GetPeers(ctx, req, grpc.Header(&header))
	if err != nil {
                fmt.Println("Error retrieving peers")
                fmt.Println(err)
		return nil, hasGateway, gateways, err
        }
	fmt.Println("Parsing peers response")
	for {
		res, err := stream.Recv()
                // If end of stream, break the loop

		if err == io.EOF {
			break
                }
                // if err, return an error
                if err != nil {
			if strings.Contains(err.Error(), "mongo: no documents in result") {
				continue
			} else {
			fmt.Println("ERROR ENCOUNTERED WITH RESPONSE")
			fmt.Println(res)
                        return peers, hasGateway, gateways, err
			}
                }
		pubkey, err := wgtypes.ParseKey(res.Peers.Publickey)
                if err != nil {
			fmt.Println("error parsing key")
                        return peers, hasGateway, gateways, err
                }

                if nodecfg.PublicKey == res.Peers.Publickey {
                        fmt.Println("Peer is self. Skipping")
                        continue
                }
                if nodecfg.Endpoint == res.Peers.Endpoint {
                        fmt.Println("Peer is self. Skipping")
                        continue
                }

		var peer wgtypes.PeerConfig
		var peeraddr = net.IPNet{
			IP: net.ParseIP(res.Peers.Address),
                        Mask: net.CIDRMask(32, 32),
		}
		var allowedips []net.IPNet
		allowedips = append(allowedips, peeraddr)
		if res.Peers.Isgateway {
			hasGateway = true
			gateways = append(gateways,res.Peers.Gatewayrange)
			_, ipnet, err := net.ParseCIDR(res.Peers.Gatewayrange)
			if err != nil {
				fmt.Println("ERROR ENCOUNTERED SETTING GATEWAY")
				fmt.Println("NOT SETTING GATEWAY")
				fmt.Println(err)
			} else {
				fmt.Println("    Gateway Range: "  + res.Peers.Gatewayrange)
				allowedips = append(allowedips, *ipnet)
			}
		}
		if keepalive != 0 {
		peer = wgtypes.PeerConfig{
			PublicKey: pubkey,
			PersistentKeepaliveInterval: &keepalivedur,
			Endpoint: &net.UDPAddr{
				IP:   net.ParseIP(res.Peers.Endpoint),
				Port: int(res.Peers.Listenport),
			},
			ReplaceAllowedIPs: true,
                        AllowedIPs: allowedips,
			}
		} else {
                peer = wgtypes.PeerConfig{
                        PublicKey: pubkey,
                        Endpoint: &net.UDPAddr{
                                IP:   net.ParseIP(res.Peers.Endpoint),
                                Port: int(res.Peers.Listenport),
                        },
                        ReplaceAllowedIPs: true,
                        AllowedIPs: allowedips,
			}
		}
		peers = append(peers, peer)

        }
	fmt.Println("Finished parsing peers response")
	return peers, hasGateway, gateways, err
}
