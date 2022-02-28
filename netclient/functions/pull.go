package functions

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"runtime"

	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/auth"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	//homedir "github.com/mitchellh/go-homedir"
)

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
	if manual {
		// check for interface change
		if cfg.Node.Interface != resNode.Interface {
			if err = DeleteInterface(cfg.Node.Interface, cfg.Node.PostDown); err != nil {
				ncutils.PrintLog("could not delete old interface "+cfg.Node.Interface, 1)
			}
		}
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
	var bkupErr = config.SaveBackup(network)
	if bkupErr != nil {
		ncutils.Log("unable to update backup file")
	}

	return &resNode, err
}
