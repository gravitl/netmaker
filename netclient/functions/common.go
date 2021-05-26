package functions

import (
	"fmt"
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
				fmt.Println(err)
			} else {
				fmt.Println("delete node " + node.MacAddress + "from remote server on network " + node.Network)
			}
		}
	}
	err = local.WipeLocal(network)
	if err != nil {
                log.Printf("Unable to wipe local config: %v", err)
	}
	err =  local.RemoveSystemDServices(network)
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
                err = local.RunCmds(runcmds)
                if err != nil {
                        fmt.Println("Error encountered running PostDown: " + err.Error())
                }
        }
        return err
}
