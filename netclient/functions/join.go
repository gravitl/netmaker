package functions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/netclientutils"
	"github.com/gravitl/netmaker/netclient/server"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc"
)

func JoinNetwork(cfg config.ClientConfig, privateKey string) error {

	hasnet := local.HasNetwork(cfg.Network)
	if hasnet {
		err := errors.New("ALREADY_INSTALLED. Netclient appears to already be installed for " + cfg.Network + ". To re-install, please remove by executing 'sudo netclient leave -n " + cfg.Network + "'. Then re-run the install command.")
		return err
	}

	netclientutils.Log("attempting to join " + cfg.Network + " at " + cfg.Server.GRPCAddress)
	err := config.Write(&cfg, cfg.Network)
	if err != nil {
		return err
	}

	if cfg.Node.Network == "" {
		return errors.New("no network provided")
	}

	if cfg.Node.LocalRange != "" && cfg.Node.LocalAddress == "" {
		log.Println("local vpn, getting local address from range: " + cfg.Node.LocalRange)
		cfg.Node.LocalAddress = getLocalIP(cfg.Node)
	}
	if cfg.Node.Password == "" {
		cfg.Node.Password = netclientutils.GenPass()
	}
	auth.StoreSecret(cfg.Node.Password, cfg.Node.Network)

	// set endpoint if blank. set to local if local net, retrieve from function if not 
	if cfg.Node.Endpoint == "" {
		if cfg.Node.IsLocal == "yes" && cfg.Node.LocalAddress != "" {
			cfg.Node.Endpoint = cfg.Node.LocalAddress
		} else {
			cfg.Node.Endpoint, err = netclientutils.GetPublicIP()

		}
		if err != nil || cfg.Node.Endpoint == "" {
			netclientutils.Log("Error setting cfg.Node.Endpoint.")
			return err
		}
	}
	// Generate and set public/private WireGuard Keys
	if privateKey == "" {
		wgPrivatekey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			log.Fatal(err)
		}
		privateKey = wgPrivatekey.String()
		cfg.Node.PublicKey = wgPrivatekey.PublicKey().String()
	}

	// Find and set node MacAddress
	if cfg.Node.MacAddress == "" {
		macs, err := netclientutils.GetMacAddr()
		if err != nil {
			return err
		} else if len(macs) == 0 {
			log.Fatal("could not retrieve mac address")
		} else {
			cfg.Node.MacAddress = macs[0]
		}
	}

	var wcclient nodepb.NodeServiceClient

	conn, err := grpc.Dial(cfg.Server.GRPCAddress, 
		netclientutils.GRPCRequestOpts(cfg.Server.GRPCSSL))

	if err != nil {
		log.Fatalf("Unable to establish client connection to "+cfg.Server.GRPCAddress+": %v", err)
	}

	wcclient = nodepb.NewNodeServiceClient(conn)

	postnode := &models.Node{
		Password:            cfg.Node.Password,
		MacAddress:          cfg.Node.MacAddress,
		AccessKey:           cfg.Server.AccessKey,
		Network:             cfg.Network,
		ListenPort:          cfg.Node.ListenPort,
		PostUp:              cfg.Node.PostUp,
		PostDown:            cfg.Node.PostDown,
		PersistentKeepalive: cfg.Node.PersistentKeepalive,
		LocalAddress:        cfg.Node.LocalAddress,
		Interface:           cfg.Node.Interface,
		PublicKey:           cfg.Node.PublicKey,
		Name:                cfg.Node.Name,
		Endpoint:            cfg.Node.Endpoint,
		SaveConfig:          cfg.Node.SaveConfig,
		UDPHolePunch:        cfg.Node.UDPHolePunch,
	}

	if err = config.ModConfig(postnode); err != nil {
		return err
	}
	data, err := json.Marshal(postnode)
	if err != nil {
		return err
	}

	// Create node on server
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

	// get free port based on returned default listen port
	node.ListenPort, err = netclientutils.GetFreePort(node.ListenPort)
	if err != nil {
		fmt.Printf("Error retrieving port: %v", err)
	}
	
	// safety check. If returned node from server is local, but not currently configured as local, set to local addr
	if cfg.Node.IsLocal != "yes" && node.IsLocal == "yes" && node.LocalRange != "" {
		node.LocalAddress, err = netclientutils.GetLocalIP(node.LocalRange)
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

	// pushing any local changes to server before starting wireguard 
	err = Push(cfg.Network)
	if err != nil {
		return err
	}

	if node.IsPending == "yes" {
		netclientutils.Log("Node is marked as PENDING.")
		netclientutils.Log("Awaiting approval from Admin before configuring WireGuard.")
		if cfg.Daemon != "off" {
			if netclientutils.IsWindows() {
				// handle daemon here..
				err = local.CreateAndRunWindowsDaemon()
			} else {
				err = local.ConfigureSystemD(cfg.Network)
			}
			return err
		}
	}

	netclientutils.Log("retrieving remote peers")
	peers, hasGateway, gateways, err := server.GetPeers(node.MacAddress, cfg.Network, cfg.Server.GRPCAddress, node.IsDualStack == "yes", node.IsIngressGateway == "yes")

	if err != nil && !netclientutils.IsEmptyRecord(err) {
		netclientutils.Log("failed to retrieve peers")
		return err
	}

	netclientutils.Log("starting wireguard")
	err = wireguard.InitWireguard(&node, privateKey, peers, hasGateway, gateways)
	if err != nil {
		return err
	}
	if cfg.Daemon != "off" {
		if netclientutils.IsWindows() {
			err = local.CreateAndRunWindowsDaemon()
		} else {
			err = local.ConfigureSystemD(cfg.Network)
		}
	}
	if err != nil {
		return err
	}

	return err
}
