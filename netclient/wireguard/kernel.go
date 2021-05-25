package wireguard

import (
	"fmt"
	"context"
        "io/ioutil"
	"strings"
	"log"
	"net"
	"os"
	"os/exec"
        "github.com/gravitl/netmaker/netclient/config"
        "github.com/gravitl/netmaker/netclient/local"
        "github.com/gravitl/netmaker/netclient/auth"
        "github.com/gravitl/netmaker/netclient/server"
        nodepb "github.com/gravitl/netmaker/grpc"
	"golang.zx2c4.com/wireguard/wgctrl"
        "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	//homedir "github.com/mitchellh/go-homedir"
)

func InitWireguard(node *nodepb.Node, privkey string, peers []wgtypes.PeerConfig, hasGateway bool, gateways []string) error  {

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
	servercfg := modcfg.Server


        if err != nil {
                log.Fatalf("failed to open client: %v", err)
        }
        defer wgclient.Close()


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
	nameserver := servercfg.Address
	nameserver = strings.Split(nameserver, ":")[0]
	network := node.Nodenetwork
        if nodecfg.Network != "" {
                network = nodecfg.Network
        } else if node.Nodenetwork != "" {
                network = node.Nodenetwork
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


	err = wgclient.ConfigureDevice(ifacename, conf)

	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Device does not exist: ")
			fmt.Println(err)
		} else {
			fmt.Printf("This is inconvenient: %v", err)
		}
	}

	//=========DNS Setup==========\\
	if nodecfg.DNS == "on" {

	        _, err := exec.LookPath("resolvectl")
		if err != nil {
			fmt.Println(err)
			fmt.Println("WARNING: resolvectl not present. Unable to set dns. Install resolvectl or run manually.")
		} else {
			_, err = exec.Command("resolvectl", "domain", ifacename, "~"+network).Output()
			if err != nil {
				fmt.Println(err)
				fmt.Println("WARNING: Error encountered setting dns. Aborted setting dns.")
			} else {
				_, err = exec.Command("resolvectl", "default-route", ifacename, "false").Output()
				if err != nil {
	                                fmt.Println(err)
	                                fmt.Println("WARNING: Error encountered setting dns. Aborted setting dns.")
				} else {
					_, err = exec.Command("resolvectl", "dns", ifacename, nameserver).Output()
					fmt.Println(err)
				}
			}
		}
	}
        //=========End DNS Setup=======\\

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
		err = local.RunCmds(runcmds)
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
                err = local.RunCmds(runcmds)
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
        if (node.Address6 != "" && node.Isdualstack) {
		fmt.Println("Adding address: " + node.Address6)
                out, err := exec.Command(ipExec, "address", "add", "dev", ifacename, node.Address6+"/64").Output()
                if err != nil {
                        fmt.Println(out)
                        fmt.Println("Error encountered adding ipv6: " + err.Error())
                }
	}
	return err
}


func SetWGKeyConfig(network string, serveraddr string) error {

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

        ctx, err = auth.SetJWT(wcclient, network)
        if err != nil {
                fmt.Printf("Failed to authenticate: %v", err)
                return err
        }

	node := server.GetNode(network)

	privatekey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return err
	}
	privkeystring := privatekey.String()
	publickey := privatekey.PublicKey()

	node.Publickey = publickey.String()

	err = StorePrivKey(privkeystring, network)
        if err != nil {
                return err
        }
        err = config.ModConfig(&node)
        if err != nil {
                return err
        }


	postnode := server.GetNode(network)

        req := &nodepb.UpdateNodeReq{
               Node: &postnode,
        }

        _, err = wcclient.UpdateNode(ctx, req, grpc.Header(&header))
        if err != nil {
                return err
        }
        err = SetWGConfig(network)
        if err != nil {
                return err
                log.Fatalf("Error: %v", err)
        }

        return err
}


func SetWGConfig(network string) error {

        cfg, err := config.ReadConfig(network)
        if err != nil {
                return err
        }
	servercfg := cfg.Server
        nodecfg := cfg.Node
        node := server.GetNode(network)

	peers, hasGateway, gateways, err := server.GetPeers(node.Macaddress, nodecfg.Network, servercfg.Address, node.Isdualstack, node.Isingressgateway)
        if err != nil {
                return err
        }
	privkey, err := RetrievePrivKey(network)
        if err != nil {
                return err
        }

        err = InitWireguard(&node, privkey, peers, hasGateway, gateways)
        if err != nil {
                return err
        }

	return err
}

func StorePrivKey(key string, network string) error{
	d1 := []byte(key)
	err := ioutil.WriteFile("/etc/netclient/wgkey-" + network, d1, 0644)
	return err
}

func RetrievePrivKey(network string) (string, error) {
	dat, err := ioutil.ReadFile("/etc/netclient/wgkey-" + network)
	return string(dat), err
}
