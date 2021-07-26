package functions

import (
        "google.golang.org/grpc/credentials"
        "crypto/tls"
	"fmt"
	"context"
	"strings"
	"log"
	"net"
	"os/exec"
        "github.com/gravitl/netmaker/netclient/config"
        "github.com/gravitl/netmaker/netclient/local"
        "github.com/gravitl/netmaker/netclient/wireguard"
        "github.com/gravitl/netmaker/netclient/server"
        "github.com/gravitl/netmaker/netclient/auth"
        nodepb "github.com/gravitl/netmaker/grpc"
        "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	//homedir "github.com/mitchellh/go-homedir"
)

func CheckIn(cliconf config.ClientConfig) error {
	network := cliconf.Network
	node := server.GetNode(network)
        cfg, err := config.ReadConfig(network)
        if err != nil {
                return err
        }
	nodecfg := cfg.Node
	servercfg := cfg.Server
	fmt.Println("Checking into server at " + servercfg.GRPCAddress)

	setupcheck := true
	ipchange := false

        if nodecfg.DNS == "on" || cliconf.Node.DNS == "on" {
		fmt.Println("setting dns")
		ifacename := node.Interface
		nameserver := servercfg.CoreDNSAddr
		network := node.Nodenetwork
                _ = local.UpdateDNS(ifacename, network, nameserver)
        }

	if !(nodecfg.IPForwarding == "off") {
		out, err := exec.Command("sysctl", "net.ipv4.ip_forward").Output()
                 if err != nil {
	                 fmt.Println(err)
			 fmt.Println("WARNING: Error encountered setting ip forwarding. This can break functionality.")
                 } else {
                         s := strings.Fields(string(out))
                         if s[2] != "1" {
				_, err = exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Output()
				if err != nil {
					fmt.Println(err)
					fmt.Println("WARNING: Error encountered setting ip forwarding. You may want to investigate this.")
				}
			}
		}
	}

	if nodecfg.Roaming != "off" {
		if nodecfg.IsLocal != "yes" {
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
		err := config.ModConfig(&node)
                if err != nil {
                        return err
                        log.Fatalf("Error: %v", err)
                }
                err = wireguard.SetWGConfig(network)
                if err != nil {
                        return err
                        log.Fatalf("Error: %v", err)
                }
	        node = server.GetNode(network)
		cfg, err := config.ReadConfig(network)
		if err != nil {
			return err
		}
		nodecfg = cfg.Node
	}

        var wcclient nodepb.NodeServiceClient
        var requestOpts grpc.DialOption
        requestOpts = grpc.WithInsecure()
        if servercfg.GRPCSSL == "on" {
		log.Println("using SSL")
                h2creds := credentials.NewTLS(&tls.Config{NextProtos: []string{"h2"}})
                requestOpts = grpc.WithTransportCredentials(h2creds)
        } else {
                log.Println("using insecure GRPC connection")
	}
        conn, err := grpc.Dial(servercfg.GRPCAddress, requestOpts)
        if err != nil {
		fmt.Printf("Cant dial GRPC server: %v", err)
		return err
        }
        wcclient = nodepb.NewNodeServiceClient(conn)

        ctx := context.Background()
        fmt.Println("Authenticating with GRPC Server")
        ctx, err = auth.SetJWT(wcclient, network)
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

                newinterface := server.GetNode(network).Interface
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
                err = wireguard.SetWGConfig(network)
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
                err = config.ModConfig(readres.Node)
                if err != nil {
			return err
                        log.Fatalf("Error: %v", err)
                }
                err = wireguard.SetWGConfig(network)
                if err != nil {
			return err
                        log.Fatalf("Error: %v", err)
                }
		setupcheck = false
	} else if nodecfg.PostChanges == "true" {
                fmt.Println("Node has requested to update remote config.")
                fmt.Println("Posting local config to remote server.")
		postnode := server.GetNode(network)
		fmt.Println("POSTING NODE: ",postnode.Macaddress,postnode.Saveconfig)
		req := &nodepb.UpdateNodeReq{
                               Node: &postnode,
                        }
		res, err := wcclient.UpdateNode(ctx, req, grpc.Header(&header))
                if err != nil {
			return err
			log.Fatalf("Error: %v", err)
                }
		res.Node.Postchanges = "false"
		err = config.ModConfig(res.Node)
                if err != nil {
			return err
                        log.Fatalf("Error: %v", err)
                }
		err = wireguard.SetWGConfig(network)
                if err != nil {
			return err
                        log.Fatalf("Error: %v", err)
                }
		setupcheck = false
	}
        if checkinres.Checkinresponse.Needkeyupdate {
                fmt.Println("Server has requested that node update key pairs.")
                fmt.Println("Proceeding to re-generate key pairs for Wiregard.")
                err = wireguard.SetWGKeyConfig(network, servercfg.GRPCAddress)
                if err != nil {
                        return err
                        log.Fatalf("Unable to process reset keys request: %v", err)
                }
                setupcheck = false
        }
        if checkinres.Checkinresponse.Needpeerupdate {
                fmt.Println("Server has requested that node update peer list.")
                fmt.Println("Updating peer list from remote server.")
                err = wireguard.SetWGConfig(network)
                if err != nil {
			return err
                        log.Fatalf("Unable to process Set Peers request: %v", err)
                }
		setupcheck = false
        }
	if checkinres.Checkinresponse.Needdelete {
		fmt.Println("This machine got the delete signal. Deleting.")
                err := LeaveNetwork(network)
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
                err = wireguard.SetWGKeyConfig(network, servercfg.GRPCAddress)
                if err != nil {
                        return err
                        log.Fatalf("Error: %v", err)
                }
	}
	}
	return nil
}

func Pull (network string) error{
        node := server.GetNode(network)
	cfg, err := config.ReadConfig(network)
        if err != nil {
                return err
        }
        servercfg := cfg.Server
        var header metadata.MD

	var wcclient nodepb.NodeServiceClient
        var requestOpts grpc.DialOption
        requestOpts = grpc.WithInsecure()
        if cfg.Server.GRPCSSL == "on" {
                h2creds := credentials.NewTLS(&tls.Config{NextProtos: []string{"h2"}})
                requestOpts = grpc.WithTransportCredentials(h2creds)
        }
        conn, err := grpc.Dial(servercfg.GRPCAddress, requestOpts)
        if err != nil {
                fmt.Printf("Cant dial GRPC server: %v", err)
                return err
        }
        wcclient = nodepb.NewNodeServiceClient(conn)

        ctx := context.Background()
        ctx, err = auth.SetJWT(wcclient, network)
        if err != nil {
                fmt.Printf("Failed to authenticate: %v", err)
                return err
        }

        req := &nodepb.ReadNodeReq{
                Macaddress: node.Macaddress,
                Network: node.Nodenetwork,
        }
         readres, err := wcclient.ReadNode(ctx, req, grpc.Header(&header))
         if err != nil {
               return err
         }
         err = config.ModConfig(readres.Node)
         if err != nil {
                return err
         }
         err = wireguard.SetWGConfig(network)
        if err != nil {
                return err
        }

	return err
}

func Push (network string) error{
        postnode := server.GetNode(network)
        cfg, err := config.ReadConfig(network)
        if err != nil {
                return err
        }
        servercfg := cfg.Server
        var header metadata.MD

        var wcclient nodepb.NodeServiceClient
        var requestOpts grpc.DialOption
        requestOpts = grpc.WithInsecure()
        if cfg.Server.GRPCSSL == "on" {
                h2creds := credentials.NewTLS(&tls.Config{NextProtos: []string{"h2"}})
                requestOpts = grpc.WithTransportCredentials(h2creds)
        }
        conn, err := grpc.Dial(servercfg.GRPCAddress, requestOpts)
        if err != nil {
                fmt.Printf("Cant dial GRPC server: %v", err)
                return err
        }
        wcclient = nodepb.NewNodeServiceClient(conn)

        ctx := context.Background()
        ctx, err = auth.SetJWT(wcclient, network)
        if err != nil {
                fmt.Printf("Failed to authenticate: %v", err)
                return err
        }

        req := &nodepb.UpdateNodeReq{
                       Node: &postnode,
                }
        _, err = wcclient.UpdateNode(ctx, req, grpc.Header(&header))
        return err
}
