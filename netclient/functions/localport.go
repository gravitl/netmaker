//go:build !freebsd
// +build !freebsd

package functions

import (
	"strconv"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.zx2c4.com/wireguard/wgctrl"
)

// GetLocalListenPort - Gets the port running on the local interface
func GetLocalListenPort(ifacename string) (int32, error) {
	client, err := wgctrl.New()
	if err != nil {
		logger.Log(0, "failed to start wgctrl")
		return 0, err
	}
	defer client.Close()
	device, err := client.Device(ifacename)
	if err != nil {
		logger.Log(0, "failed to parse interface", ifacename)
		return 0, err
	}
	return int32(device.ListenPort), nil
}

// UpdateLocalListenPort - check local port, if different, mod config and publish
func UpdateLocalListenPort(nodeCfg *config.ClientConfig) error {
	var err error
	ifacename := getRealIface(nodeCfg.Node.Interface, nodeCfg.Node.Address)
	localPort, err := GetLocalListenPort(ifacename)
	if err != nil {
		logger.Log(1, "network:", nodeCfg.Node.Network, "error encountered checking local listen port: ", ifacename, err.Error())
	} else if nodeCfg.Node.LocalListenPort != localPort && localPort != 0 {
		logger.Log(1, "network:", nodeCfg.Node.Network, "local port has changed from ", strconv.Itoa(int(nodeCfg.Node.LocalListenPort)), " to ", strconv.Itoa(int(localPort)))
		nodeCfg.Node.LocalListenPort = localPort
		err = config.ModNodeConfig(&nodeCfg.Node)
		if err != nil {
			return err
		}
		if err := PublishNodeUpdate(nodeCfg); err != nil {
			logger.Log(0, "could not publish local port change", err.Error())
		}
	}
	return err
}

func getRealIface(ifacename string, address string) string {
	var deviceiface = ifacename
	var err error
	if ncutils.IsMac() { // if node is Mac (Darwin) get the tunnel name first
		deviceiface, err = local.GetMacIface(address)
		if err != nil || deviceiface == "" {
			deviceiface = ifacename
		}
	}
	return deviceiface
}
