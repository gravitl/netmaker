package functions

import (
        "google.golang.org/grpc/credentials"
        "crypto/tls"
	"fmt"
	"encoding/json"
	"errors"
	"context"
        "net/http"
        "io/ioutil"
	"strings"
	"log"
	"net"
	"os"
	"os/exec"
        "github.com/gravitl/netmaker/netclient/config"
        "github.com/gravitl/netmaker/netclient/local"
        "github.com/gravitl/netmaker/netclient/auth"
        nodepb "github.com/gravitl/netmaker/grpc"
	"golang.zx2c4.com/wireguard/wgctrl"
        "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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

	iplist := []string{"http://ip.client.gravitl.com","https://ifconfig.me", "http://api.ipify.org", "http://ipinfo.io/ip"}
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

func GetNode(network string) nodepb.Node {

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
	node.Address6 = nodecfg.WGAddress6
	node.Listenport = nodecfg.Port
	node.Keepalive = nodecfg.KeepAlive
	node.Postup = nodecfg.PostUp
	node.Postdown = nodecfg.PostDown
	node.Publickey = nodecfg.PublicKey
	node.Macaddress = nodecfg.MacAddress
	node.Endpoint = nodecfg.Endpoint
	node.Password = nodecfg.Password
        if nodecfg.DNS == "on" {
                node.Dnsoff = true
        } else {
                node.Dnsoff = false
        }
	if nodecfg.IsDualStack == "yes" {
		node.Isdualstack = true
	} else {
		node.Isdualstack = false
	}
        if nodecfg.IsIngressGateway == "yes" {
                node.Isingressgateway = true
        } else {
                node.Isingressgateway = false
        }
        return node
}

func Uninstall() error {
	networks, err := GetNetworks()
	if err != nil {
		log.Println("unable to retrieve networks: ", err)
		log.Println("continuing uninstall without leaving networks")
	} else {
		for _, network := range networks {
			err = LeaveNetwork(network)
			if err != nil {
				log.Println("Encounter issue leaving network " + network + ": ", err)
			}
		}
	}
	return err
}

func LeaveNetwork(network string) error {
        //need to  implement checkin on server side
        cfg, err := config.ReadConfig(network)
        if err != nil {
                return err
        }
	servercfg := cfg.Server
        node := cfg.Node

        var wcclient nodepb.NodeServiceClient
        var requestOpts grpc.DialOption
        requestOpts = grpc.WithInsecure()
        if cfg.Server.GRPCSSL == "on" {
                h2creds := credentials.NewTLS(&tls.Config{NextProtos: []string{"h2"}})
                requestOpts = grpc.WithTransportCredentials(h2creds)
        }
        conn, err := grpc.Dial(servercfg.GRPCAddress, requestOpts)
	if err != nil {
                log.Printf("Unable to establish client connection to " + servercfg.GRPCAddress + ": %v", err)
        }else {
		wcclient = nodepb.NewNodeServiceClient(conn)

		ctx := context.Background()
		ctx, err = auth.SetJWT(wcclient, network)
		if err != nil {
                log.Printf("Failed to authenticate: %v", err)
		} else {
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
				log.Println(err)
			} else {
				log.Println("Removed machine from " + node.Network + " network on remote server")
			}
		}
	}
	err = local.WipeLocal(network)
	if err != nil {
                log.Printf("Unable to wipe local config: %v", err)
	} else {
		log.Println("Removed " + node.Network + " network locally")
	}
	if cfg.Daemon != "off" {
		err =  local.RemoveSystemDServices(network)
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
                log.Println(err)
        }
        if postdown != "" {
                runcmds := strings.Split(postdown, "; ")
                err = local.RunCmds(runcmds)
                if err != nil {
                        log.Println("Error encountered running PostDown: " + err.Error())
                }
        }
        return err
}


func List() error{

	networks, err := GetNetworks()
	if err != nil {
		return err
	}
	for _, network := range networks {
		cfg, err := config.ReadConfig(network)
		if err == nil {
			//cfg2 := *cfg
			listconfig := &config.ListConfig{
					Name: cfg.Node.Name,
					Interface: cfg.Node.Interface,
					PrivateIPv4: cfg.Node.WGAddress,
					PrivateIPv6: cfg.Node.WGAddress6,
					PublicEndpoint: cfg.Node.Endpoint,
				}
			jsoncfg, _ := json.Marshal(listconfig)
			log.Println(network + ": " + string(jsoncfg))
		} else {
			log.Println(network + ": Could not retrieve network configuration.")
		}
	}
	return nil

}

func GetNetworks() ([]string, error) {
        var networks []string
        files, err := ioutil.ReadDir("/etc/netclient")
        if err != nil {
                return networks, err
        }
        for _, f := range files {
                if strings.Contains(f.Name(), "netconfig-") && !strings.Contains(f.Name(), "global-001"){
                        networkname := stringAfter(f.Name(), "netconfig-")
                        networks = append(networks, networkname)
                }
        }
	return networks, err
}

func stringAfter(original string, substring string) string {
	position := strings.LastIndex(original, substring)
	if position == -1 {
		return ""
	}
	adjustedPosition := position + len(substring)

	if adjustedPosition >= len(original) {
		return ""
	}
	return original[adjustedPosition:len(original)]
}
