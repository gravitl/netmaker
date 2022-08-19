package functions

import (
	"fmt"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/wireguard"
)

// Connect - will attempt to connect a node on given network
func Connect(network string) error {
	cfg, err := config.ReadConfig(network)
	if err != nil {
		return err
	}
	if cfg.Node.Connected == "yes" {
		return fmt.Errorf("node already connected")
	}
	cfg.Node.Connected = "yes"
	filePath := ncutils.GetNetclientPathSpecific() + cfg.Node.Interface + ".conf"

	if err = wireguard.ApplyConf(&cfg.Node, cfg.Node.Interface, filePath); err != nil {
		return err
	}
	if err := PublishNodeUpdate(cfg); err != nil {
		logger.Log(0, "network:", cfg.Node.Network, "could not publish connection change, it will likely get reverted")
	}

	return config.ModNodeConfig(&cfg.Node)
}

// Disconnect - attempts to disconnect a node on given network
func Disconnect(network string) error {
	cfg, err := config.ReadConfig(network)
	if err != nil {
		return err
	}
	if cfg.Node.Connected == "no" {
		return fmt.Errorf("node already disconnected")
	}
	cfg.Node.Connected = "no"
	filePath := ncutils.GetNetclientPathSpecific() + cfg.Node.Interface + ".conf"

	if err = wireguard.ApplyConf(&cfg.Node, cfg.Node.Interface, filePath); err != nil {
		return err
	}
	if err := PublishNodeUpdate(cfg); err != nil {
		logger.Log(0, "network:", cfg.Node.Network, "could not publish connection change, it will likely get reverted")
	}

	return config.ModNodeConfig(&cfg.Node)
}
