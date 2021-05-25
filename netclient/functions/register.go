package functions

import (
	"fmt"
	"errors"
	"context"
	"log"
	"net"
	"strconv"
        "github.com/gravitl/netmaker/netclient/config"
        "github.com/gravitl/netmaker/netclient/wireguard"
        "github.com/gravitl/netmaker/netclient/server"
        "github.com/gravitl/netmaker/netclient/local"
        nodepb "github.com/gravitl/netmaker/grpc"
	"golang.zx2c4.com/wireguard/wgctrl"
        "google.golang.org/grpc"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	//homedir "github.com/mitchellh/go-homedir"
)

func Register(cfg config.ClientConfig) error {

        if err != nil {
                log.Fatalf("Unable to establish client connection to localhost:50051: %v", err)
        }
        wcclient = nodepb.NewNodeServiceClient(conn)

        postclient := &models.ServerClient{
                AccessKey: cfg.Server.AccessKey,
                Publickey: cfg.Node.PublicKey,
                Privatekey: cfg.Node.PublicKey,
		Address: cfg.Node.Address,
		Address6: cfg.Node.Address6,
		Network: "comms"
	}
	bytes, err := json.Marshal(postclient)
	body := bytes.NewBuffer(bytes)
	res, err := http.Post("http://"+cfg.Server.Address+""jsonplaceholder.typicode.com/posts/1")
        if err != nil {
                return err
        }
        node := res.Node
        if err != nil {
                return err
        }

       fmt.Println("Node Settings: ")
       fmt.Println("     Password: " + node.Password)
       fmt.Println("     WG Address: " + node.Address)
       fmt.Println("     WG ipv6 Address: " + node.Address6)
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
       fmt.Println("     Is Dual Stack?: " + strconv.FormatBool(node.Isdualstack))
       fmt.Println("     Is Ingress Gateway?: " + strconv.FormatBool(node.Isingressgateway))
       fmt.Println("     Local Range: " + node.Localrange)

       if node.Dnsoff==true  {
		cfg.Node.DNS = "yes"
	}
	if !(cfg.Node.IsLocal == "yes") && node.Islocal && node.Localrange != "" {
		node.Localaddress, err = getLocalIP(node.Localrange)
		if err != nil {
			return err
		}
		node.Endpoint = node.Localaddress
	}

        err = config.ModConfig(node)
        if err != nil {
                return err
        }

	if node.Ispending {
		fmt.Println("Node is marked as PENDING.")
		fmt.Println("Awaiting approval from Admin before configuring WireGuard.")
	        if cfg.Daemon != "no" {
			fmt.Println("Configuring Netmaker Service.")
			err = local.ConfigureSystemD(cfg.Network)
			return err
		}
	}

	peers, hasGateway, gateways, err := server.GetPeers(node.Macaddress, cfg.Network, cfg.Server.Address, node.Isdualstack, node.Isingressgateway)

	if err != nil {
                return err
        }
	err = wireguard.StorePrivKey(cfg.Node.PrivateKey, cfg.Network)
        if err != nil {
                return err
        }
	err = wireguard.InitWireguard(node, cfg.Node.PrivateKey, peers, hasGateway, gateways)
        if err != nil {
                return err
        }
	if cfg.Daemon == "off" {
		err = local.ConfigureSystemD(cfg.Network)
	}
        if err != nil {
                return err
        }

	return err
}
