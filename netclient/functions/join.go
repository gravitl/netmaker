package functions

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/gravitl/netmaker/database"
	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/server"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	//homedir "github.com/mitchellh/go-homedir"
)

func JoinNetwork(cfg config.ClientConfig, privateKey string) error {

	hasnet := local.HasNetwork(cfg.Network)
	if hasnet {
		err := errors.New("ALREADY_INSTALLED. Netclient appears to already be installed for " + cfg.Network + ". To re-install, please remove by executing 'sudo netclient leave -n " + cfg.Network + "'. Then re-run the install command.")
		return err
	}
	log.Println("attempting to join " + cfg.Network + " at " + cfg.Server.GRPCAddress)
	err := config.Write(&cfg, cfg.Network)
	if err != nil {
		return err
	}

	wgclient, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer wgclient.Close()
	if cfg.Node.Network == "" {
		return errors.New("no network provided")
	}
	if cfg.Node.LocalRange != "" {
		if cfg.Node.LocalAddress == "" {
			log.Println("local vpn, getting local address from range: " + cfg.Node.LocalRange)
			ifaces, err := net.Interfaces()
			if err != nil {
				return err
			}
			_, localrange, err := net.ParseCIDR(cfg.Node.LocalRange)
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
							if cfg.Node.IsLocal == "yes" {
								found = localrange.Contains(ip)
							} else {
								found = true
							}
						}
					case *net.IPAddr:
						if !found {
							ip = v.IP
							local = ip.String()
							if cfg.Node.IsLocal == "yes" {
								found = localrange.Contains(ip)

							} else {
								found = true
							}
						}
					}
				}
			}
			cfg.Node.LocalAddress = local
		}
	}
	if cfg.Node.Password == "" {
		cfg.Node.Password = GenPass()
	}
	auth.StoreSecret(cfg.Node.Password, cfg.Node.Network)
	if cfg.Node.Endpoint == "" {
		if cfg.Node.IsLocal == "yes" && cfg.Node.LocalAddress != "" {
			cfg.Node.Endpoint = cfg.Node.LocalAddress
		} else {
			cfg.Node.Endpoint, err = getPublicIP()
			if err != nil {
				fmt.Println("Error setting cfg.Node.Endpoint.")
				return err
			}
		}
	}
	if privateKey == "" {
		wgPrivatekey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			log.Fatal(err)
		}
		privateKey = wgPrivatekey.String()
		cfg.Node.PublicKey = wgPrivatekey.PublicKey().String()
	}

	if cfg.Node.MacAddress == "" {
		macs, err := getMacAddr()
		if err != nil {
			return err
		} else if len(macs) == 0 {
			log.Fatal()
		} else {
			cfg.Node.MacAddress = macs[0]
		}
	}
	if cfg.Node.ListenPort == 0 {
		cfg.Node.ListenPort, err = GetFreePort(51821)
		if err != nil {
			fmt.Printf("Error retrieving port: %v", err)
		}
	}
	var wcclient nodepb.NodeServiceClient
	var requestOpts grpc.DialOption
	requestOpts = grpc.WithInsecure()
	if cfg.Server.GRPCSSL == "on" {
		h2creds := credentials.NewTLS(&tls.Config{NextProtos: []string{"h2"}})
		requestOpts = grpc.WithTransportCredentials(h2creds)
	}
	conn, err := grpc.Dial(cfg.Server.GRPCAddress, requestOpts)

	if err != nil {
		log.Fatalf("Unable to establish client connection to "+cfg.Server.GRPCAddress+": %v", err)
	}

	wcclient = nodepb.NewNodeServiceClient(conn)

	// postnode := &models.Node{
	// 	Password:            cfg.Node.Password,
	// 	MacAddress:          cfg.Node.MacAddress,
	// 	AccessKey:           cfg.Server.AccessKey,
	// 	Network:             cfg.Network,
	// 	ListenPort:          cfg.Node.ListenPort,
	// 	PostUp:              cfg.Node.PostUp,
	// 	PostDown:            cfg.Node.PostDown,
	// 	PersistentKeepalive: cfg.Node.PersistentKeepalive,
	// 	LocalAddress:        cfg.Node.LocalAddress,
	// 	Interface:           cfg.Node.Interface,
	// 	PublicKey:           cfg.Node.PublicKey,
	// 	Name:                cfg.Node.Name,
	// 	Endpoint:            cfg.Node.Endpoint,
	// 	SaveConfig:          cfg.Node.SaveConfig,
	// 	UDPHolePunch:        cfg.Node.UDPHolePunch,
	// }
	if err = config.ModConfig(&cfg.Node); err != nil {
		return err
	}
	data, err := json.Marshal(&cfg.Node)
	if err != nil {
		return err
	}

	res, err := wcclient.CreateNode(
		context.TODO(),
		&nodepb.Object{
			Data: string(data),
			Type: nodepb.NODE_TYPE,
		},
	)
	if err != nil {
		return err
	}
	log.Println("node created on remote server...updating configs")

	nodeData := res.Data
	var node models.Node
	if err = json.Unmarshal([]byte(nodeData), &node); err != nil {
		return err
	}

	if node.DNSOn == "yes" {
		cfg.Node.DNSOn = "yes"
	}
	if !(cfg.Node.IsLocal == "yes") && node.IsLocal == "yes" && node.LocalRange != "" {
		node.LocalAddress, err = getLocalIP(node.LocalRange)
		if err != nil {
			return err
		}
		node.Endpoint = node.LocalAddress
	}
	err = config.ModConfig(&node)
	if err != nil {
		return err
	}

	err = wireguard.StorePrivKey(privateKey, cfg.Network)
	if err != nil {
		return err
	}

	if node.IsPending == "yes" {
		fmt.Println("Node is marked as PENDING.")
		fmt.Println("Awaiting approval from Admin before configuring WireGuard.")
		if cfg.Daemon != "off" {
			err = local.ConfigureSystemD(cfg.Network)
			return err
		}
	}
	log.Println("retrieving remote peers")
	peers, hasGateway, gateways, err := server.GetPeers(node.MacAddress, cfg.Network, cfg.Server.GRPCAddress, node.IsDualStack == "yes", node.IsIngressGateway == "yes")

	if err != nil && !database.IsEmptyRecord(err) {
		log.Println("failed to retrieve peers", err)
		return err
	}

	log.Println("starting wireguard")
	err = wireguard.InitWireguard(&node, privateKey, peers, hasGateway, gateways)
	if err != nil {
		return err
	}
	if cfg.Daemon != "off" {
		err = local.ConfigureSystemD(cfg.Network)
	}
	if err != nil {
		return err
	}

	return err
}

//generate an access key value
func GenPass() string {

	var seededRand *rand.Rand = rand.New(
		rand.NewSource(time.Now().UnixNano()))

	length := 16
	charset := "abcdefghijklmnopqrstuvwxyz" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
