package functions

import (
	"context"
	"crypto/ed25519"

	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"runtime"

	"filippo.io/edwards25519"
	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/daemon"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/server"
	"github.com/gravitl/netmaker/netclient/wireguard"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func Register(cfg *config.ClientConfig, registrationServer string) (err error) {
	var pubk ed25519.PublicKey
	var seck ed25519.PrivateKey
	var signature [ed25519.SignatureSize]byte
	var curve25519Point *edwards25519.Point
	var wgKey wgtypes.Key
	var node *models.Node = nodeFrom(cfg)
	var wcclient nodepb.NodeServiceClient
	// var params *models.AuthParams = &models.AuthParams{
	// 	MacAddress: macOrRNG(),
	// 	ID:         ncutils.GetHostname(),
	// 	Password:   "REMOVE_AFTER_TESTING",
	// 	PublicKey:  pubk,
	// }
	// origin, err := ncutils.GetPublicIP()
	// if err != nil {
	// 	origin = "localhost"
	// }
	if seck, err = findOrGenerateAuthKeys(cfg); err != nil {
		return
	}

	pubk = seck.Public().(ed25519.PublicKey)

	node.PublicAuthKey = pubk

	rawNode, err := json.Marshal(&node)
	if err != nil {
		return
	}
	copy(signature[:], ed25519.Sign(seck, rawNode))

	if !ed25519.Verify(pubk, rawNode, signature[:]) {
		err = errors.New("failed to verify signature on self-signed request")
		return
	}

	res, err := wcclient.CreateNode(
		context.TODO(),
		&nodepb.Object{
			Data:      rawNode,
			Type:      nodepb.NODE_TYPE,
			Signature: signature[:],
		},
	)

	currentListenPort := node.ListenPort
	if err = json.Unmarshal(res.Data, node); err != nil {
		return
	}
	cfg.Node = *node
	setListenPort(currentListenPort, cfg)
	if err = config.ModConfig(node); err != nil {
		return
	}

	// attempt to make backup
	if err = config.SaveBackup(node.Network); err != nil {
		logger.Log(0, "failed to make backup, node will not auto restore if config is corrupted")
	}

	logger.Log(0, "retrieving peers")
	peers, hasGateway, gateways, err := server.GetPeers(node.MacAddress, cfg.Network, cfg.Server.GRPCAddress, node.IsDualStack == "yes", node.IsIngressGateway == "yes", node.IsServer == "yes")
	if err != nil && !ncutils.IsEmptyRecord(err) {
		logger.Log(0, "failed to retrieve peers")
		return
	}

	logger.Log(0, "starting wireguard")

	curve25519Point, err = (&edwards25519.Point{}).SetBytes(seck)
	if err != nil {
		logger.Log(0, "failed to transform twisted edwards point to montogomery form")
		logger.Log(0, "generating new key-pair for wireguard instead")
		wgKey, err = wgtypes.GenerateKey()
		if err != nil {
			return
		}
	} else {
		wgKey, err = wgtypes.ParseKey(string(curve25519Point.BytesMontgomery()))
		if err != nil {
			wgKey, err = wgtypes.GenerateKey()
			if err != nil {
				return
			}
		}
	}

	err = wireguard.InitWireguard(node, wgKey.String(), peers, hasGateway, gateways, false)
	if cfg.Daemon != "off" {
		if err = daemon.InstallDaemon(cfg); err != nil {
			return
		}
		logger.Log(0, "you made it")
		err = daemon.Restart()
	}
	return
}

func macOrRNG() string {
	macAddrs, err := ncutils.GetMacAddr()
	if err != nil || len(macAddrs) < 1 {
		macAddrs = []string{
			ncutils.MakeRandomString(18),
		}
	}
	return macAddrs[0]
}

func findOrGenerateAuthKeys(cfg *config.ClientConfig) (seck ed25519.PrivateKey, err error) {
	if privateKeyPath := authKeyPath(cfg); !config.FileExists(privateKeyPath) {
		_, seck, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return
		}
		if err = os.WriteFile(privateKeyPath, seck, 077); err != nil {
			return
		}
	} else {
		seck, err = os.ReadFile(privateKeyPath)
	}
	return
}

func authKeyPath(cfg *config.ClientConfig) string {
	return "/etc/netclient/id_ed25518-" + cfg.Node.Name
}

func nodeFrom(cfg *config.ClientConfig) *models.Node {
	return &models.Node{
		Password:   cfg.Node.Password,
		Address:    cfg.Node.Address,
		Address6:   cfg.Node.Address6,
		ID:         cfg.Node.ID,
		MacAddress: cfg.Node.MacAddress,
		AccessKey:  cfg.Server.AccessKey,
		IsStatic:   cfg.Node.IsStatic,
		//Roaming:             cfg.Node.Roaming,
		Network:             cfg.Network,
		ListenPort:          cfg.Node.ListenPort,
		PostUp:              cfg.Node.PostUp,
		PostDown:            cfg.Node.PostDown,
		PersistentKeepalive: cfg.Node.PersistentKeepalive,
		LocalAddress:        cfg.Node.LocalAddress,
		Interface:           cfg.Node.Interface,
		PublicKey:           cfg.Node.PublicKey,
		DNSOn:               cfg.Node.DNSOn,
		Name:                cfg.Node.Name,
		Endpoint:            cfg.Node.Endpoint,
		UDPHolePunch:        cfg.Node.UDPHolePunch,
		TrafficKeys:         cfg.Node.TrafficKeys,
		OS:                  runtime.GOOS,
		Version:             ncutils.Version,
	}
}
