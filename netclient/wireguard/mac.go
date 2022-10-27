package wireguard

import (
	"errors"
	"fmt"
	"strings"

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
	realIfaceName = strings.TrimSpace(realIfaceName)
	if !(ncutils.FileExists(fmt.Sprintf("/var/run/wireguard/%s.sock", realIfaceName))) {
		return "", errors.New("interface file does not exist")
	}
	return realIfaceName, nil
}
