package functions

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/local"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// Get LocalListenPort - Gets the port running on the local interface
func GetLocalListenPort(ifacename string) (int32, error) {
	portstring, err := ncutils.RunCmd("wg show "+ifacename+" listen-port", false)
	if err != nil {
		return 0, err
	}
	portstring = strings.TrimSuffix(portstring, "\n")
	i, err := strconv.ParseInt(portstring, 10, 32)
	if err != nil {
		return 0, err
	} else if i == 0 {
		return 0, errors.New("parsed port is unset or invalid")
	}
	return int32(i), nil
}

// UpdateLocalListenPort - check local port, if different, mod config and publish
func UpdateLocalListenPort(nodeCfg *config.ClientConfig) error {
	var err error
	ifacename := getRealIface(nodeCfg.Node.Interface, nodeCfg.Node.Address)
	localPort, err := GetLocalListenPort(ifacename)
	if err != nil {
		logger.Log(1, "error encountered checking local listen port: ", err.Error())
	} else if nodeCfg.Node.LocalListenPort != localPort && localPort != 0 {
		logger.Log(1, "local port has changed from ", strconv.Itoa(int(nodeCfg.Node.LocalListenPort)), " to ", strconv.Itoa(int(localPort)))
		nodeCfg.Node.LocalListenPort = localPort
		err = config.ModConfig(&nodeCfg.Node)
		if err != nil {
			return err
		}
		if err := PublishNodeUpdate(nodeCfg); err != nil {
			logger.Log(0, "could not publish local port change")
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
