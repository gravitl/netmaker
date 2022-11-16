package wireguard

import (
	"errors"

	"github.com/gravitl/netmaker/netclient/ncutils"
)

// GetRealIface - retrieves tun iface based on reference iface name from config file
func GetRealIface(iface string) (string, error) {
	ncutils.RunCmd("wg show interfaces", false)
	ifacePath := "/var/run/wireguard/" + iface + ".name"
	if !(ncutils.FileExists(ifacePath)) {
		return "", errors.New(ifacePath + " does not exist")
	}
	realIfaceName, err := ncutils.GetFileAsString(ifacePath)
	if err != nil {
		return "", err
	}
	if !(ncutils.FileExists("/var/run/wireguard/" + realIfaceName + ".sock")) {
		return "", errors.New("interface file does not exist")
	}
	return realIfaceName, nil
}
