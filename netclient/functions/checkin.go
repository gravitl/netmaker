package functions

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"strings"

	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	//homedir "github.com/mitchellh/go-homedir"
)

func isDeleteError(err error) bool {
	return err != nil && strings.Contains(err.Error(), models.NODE_DELETE)
}

func checkIP(node *models.Node, servercfg config.ServerConfig, cliconf config.ClientConfig, network string) bool {
	ipchange := false
	var err error
	if node.Roaming == "yes" && node.IsStatic != "yes" {
		if node.IsLocal == "no" {
			extIP, err := ncutils.GetPublicIP()
			if err != nil {
				ncutils.PrintLog("error encountered checking ip addresses: "+err.Error(), 1)
			}
			if node.Endpoint != extIP && extIP != "" {
				ncutils.PrintLog("endpoint has changed from "+
					node.Endpoint+" to "+extIP, 1)
				ncutils.PrintLog("updating address", 1)
				node.Endpoint = extIP
				ipchange = true
			}
			intIP, err := getPrivateAddr()
			if err != nil {
				ncutils.PrintLog("error encountered checking ip addresses: "+err.Error(), 1)
			}
			if node.LocalAddress != intIP && intIP != "" {
				ncutils.PrintLog("local Address has changed from "+
					node.LocalAddress+" to "+intIP, 1)
				ncutils.PrintLog("updating address", 1)
				node.LocalAddress = intIP
				ipchange = true
			}
		} else {
			localIP, err := ncutils.GetLocalIP(node.LocalRange)
			if err != nil {
				ncutils.PrintLog("error encountered checking ip addresses: "+err.Error(), 1)
			}
			if node.Endpoint != localIP && localIP != "" {
				ncutils.PrintLog("endpoint has changed from "+
					node.Endpoint+" to "+localIP, 1)
				ncutils.PrintLog("updating address", 1)
				node.Endpoint = localIP
				node.LocalAddress = localIP
				ipchange = true
			}
		}
	}
	if ipchange {
		err = config.ModConfig(node)
		if err != nil {
			ncutils.PrintLog("error modifying config file: "+err.Error(), 1)
			return false
		}
		err = wireguard.SetWGConfig(network, false)
		if err != nil {
			ncutils.PrintLog("error setting wireguard config: "+err.Error(), 1)
			return false
		}
	}
	return ipchange && err == nil
}

// DEPRECATED
// func setDNS(node *models.Node, servercfg config.ServerConfig, nodecfg *models.Node) {
// 	if nodecfg.DNSOn == "yes" {
// 		ifacename := node.Interface
// 		nameserver := servercfg.CoreDNSAddr
// 		network := node.Network
// 		local.UpdateDNS(ifacename, network, nameserver)
// 	}
// }

func checkNodeActions(node *models.Node, networkName string, servercfg config.ServerConfig, localNode *models.Node, cfg *config.ClientConfig) string {
	if (node.Action == models.NODE_UPDATE_KEY || localNode.Action == models.NODE_UPDATE_KEY) &&
		node.IsStatic != "yes" {
		err := wireguard.SetWGKeyConfig(networkName, servercfg.GRPCAddress)
		if err != nil {
			ncutils.PrintLog("unable to process reset keys request: "+err.Error(), 1)
			return ""
		}
	}
	if node.Action == models.NODE_DELETE || localNode.Action == models.NODE_DELETE {
		err := RemoveLocalInstance(cfg, networkName)
		if err != nil {
			ncutils.PrintLog("error deleting locally: "+err.Error(), 1)
		}
		return models.NODE_DELETE
	}
	return ""
}

// CheckConfig - checks if current config of client needs update, see flow below
/**
 * Pull changes if any (interface refresh)
 * - Save it
 * Check local changes for (ipAddress, publickey, configfile changes) (interface refresh)
 * - Save it
 * - Push it
 * Pull Peers (sync)
 */
func CheckConfig(cliconf config.ClientConfig) error {

	network := cliconf.Network
	cfg, err := config.ReadConfig(network)
	if err != nil {
		return err
	}
	servercfg := cfg.Server
	currentNode := cfg.Node

	newNode, err := Pull(network, false)
	if isDeleteError(err) {
		return RemoveLocalInstance(cfg, network)
	}
	if err != nil {
		return err
	}
	if newNode.IsPending == "yes" {
		return errors.New("node is pending")
	}
	actionCompleted := checkNodeActions(newNode, network, servercfg, &currentNode, cfg)
	if actionCompleted == models.NODE_DELETE {
		return errors.New("node has been removed")
	}
	// Check if ip changed and push if so
	checkIP(newNode, servercfg, cliconf, network)
	return Push(network)
}

// Pull - pulls the latest config from the server, if manual it will overwrite
func Pull(network string, manual bool) (*models.Node, error) {
	cfg, err := config.ReadConfig(network)
	if err != nil {
		return nil, err
	}

	node := cfg.Node
	//servercfg := cfg.Server

	if cfg.Node.IPForwarding == "yes" && !ncutils.IsWindows() {
		if err = local.SetIPForwarding(); err != nil {
			return nil, err
		}
	}
	var resNode models.Node // just need to fill this with either server calls or client calls

	var header metadata.MD
	var wcclient nodepb.NodeServiceClient
	var ctx context.Context

	if cfg.Node.IsServer != "yes" {
		conn, err := grpc.Dial(cfg.Server.GRPCAddress,
			ncutils.GRPCRequestOpts(cfg.Server.GRPCSSL))
		if err != nil {
			ncutils.PrintLog("Cant dial GRPC server: "+err.Error(), 1)
			return nil, err
		}
		defer conn.Close()
		wcclient = nodepb.NewNodeServiceClient(conn)

		ctx, err = auth.SetJWT(wcclient, network)
		if err != nil {
			ncutils.PrintLog("Failed to authenticate: "+err.Error(), 1)
			return nil, err
		}

		data, err := json.Marshal(&node)
		if err != nil {
			ncutils.PrintLog("Failed to parse node config: "+err.Error(), 1)
			return nil, err
		}

		req := &nodepb.Object{
			Data: string(data),
			Type: nodepb.NODE_TYPE,
		}

		readres, err := wcclient.ReadNode(ctx, req, grpc.Header(&header))
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(readres.Data), &resNode); err != nil {
			return nil, err
		}
	}
	// ensure that the OS never changes
	resNode.OS = runtime.GOOS
	if resNode.PullChanges == "yes" || manual {
		// check for interface change
		if cfg.Node.Interface != resNode.Interface {
			if err = DeleteInterface(cfg.Node.Interface, cfg.Node.PostDown); err != nil {
				ncutils.PrintLog("could not delete old interface "+cfg.Node.Interface, 1)
			}
		}
		resNode.PullChanges = "no"
		if err = config.ModConfig(&resNode); err != nil {
			return nil, err
		}
		if err = wireguard.SetWGConfig(network, false); err != nil {
			return nil, err
		}
		nodeData, err := json.Marshal(&resNode)
		if err != nil {
			return &resNode, err
		}
		if resNode.IsServer != "yes" {
			if wcclient == nil || ctx == nil {
				return &cfg.Node, errors.New("issue initializing gRPC client")
			}
			req := &nodepb.Object{
				Data:     string(nodeData),
				Type:     nodepb.NODE_TYPE,
				Metadata: "",
			}
			_, err = wcclient.UpdateNode(ctx, req, grpc.Header(&header))
			if err != nil {
				return &resNode, err
			}
		}
	} else {
		if err = wireguard.SetWGConfig(network, true); err != nil {
			if errors.Is(err, os.ErrNotExist) && !ncutils.IsFreeBSD() {
				return Pull(network, true)
			} else {
				return nil, err
			}
		}
	}
	//if ncutils.IsLinux() {
	//	setDNS(&resNode, servercfg, &cfg.Node)
	//}
	var bkupErr = config.SaveBackup(network)
	if bkupErr != nil {
		ncutils.Log("unable to update backup file")
	}

	return &resNode, err
}

// Push - pushes current client configuration to server
func Push(network string) error {

	cfg, err := config.ReadConfig(network)
	if err != nil {
		return err
	}
	postnode := cfg.Node
	// always set the OS on client
	postnode.OS = runtime.GOOS
	postnode.SetLastCheckIn()

	var header metadata.MD
	var wcclient nodepb.NodeServiceClient
	conn, err := grpc.Dial(cfg.Server.GRPCAddress,
		ncutils.GRPCRequestOpts(cfg.Server.GRPCSSL))
	if err != nil {
		ncutils.PrintLog("Cant dial GRPC server: "+err.Error(), 1)
		return err
	}
	defer conn.Close()
	wcclient = nodepb.NewNodeServiceClient(conn)

	ctx, err := auth.SetJWT(wcclient, network)
	if err != nil {
		ncutils.PrintLog("Failed to authenticate with server: "+err.Error(), 1)
		return err
	}
	if postnode.IsPending != "yes" {
		privateKey, err := wireguard.RetrievePrivKey(network)
		if err != nil {
			return err
		}
		privateKeyWG, err := wgtypes.ParseKey(privateKey)
		if err != nil {
			return err
		}
		if postnode.PublicKey != privateKeyWG.PublicKey().String() {
			postnode.PublicKey = privateKeyWG.PublicKey().String()
		}
	}
	nodeData, err := json.Marshal(&postnode)
	if err != nil {
		return err
	}

	req := &nodepb.Object{
		Data:     string(nodeData),
		Type:     nodepb.NODE_TYPE,
		Metadata: "",
	}
	data, err := wcclient.UpdateNode(ctx, req, grpc.Header(&header))
	if err != nil {
		return err
	}
	err = json.Unmarshal([]byte(data.Data), &postnode)
	if err != nil {
		return err
	}
	err = config.ModConfig(&postnode)
	return err
}
