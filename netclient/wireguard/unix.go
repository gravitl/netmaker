package wireguard

import (
	"fmt"
	"os"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// ApplyWGQuickConf - applies wg-quick commands if os supports
func ApplyWGQuickConf(confPath, ifacename string, isConnected bool) error {
	if ncutils.IsWindows() {
		return ApplyWindowsConf(confPath, isConnected)
	} else {
		_, err := os.Stat(confPath)
		if err != nil {
			logger.Log(0, confPath+" does not exist "+err.Error())
			return err
		}
		if ncutils.IfaceExists(ifacename) {
			ncutils.RunCmd("wg-quick down "+confPath, true)
		}
		if !isConnected {
			return nil
		}
		_, err = ncutils.RunCmd("wg-quick up "+confPath, true)

		return err
	}
}

// ApplyMacOSConf - applies system commands similar to wg-quick using golang for MacOS
func ApplyMacOSConf(node *models.Node, ifacename, confPath string, isConnected bool) error {
	var err error
	_ = WgQuickDownMac(node, ifacename)
	if !isConnected {
		return nil
	}
	err = WgQuickUpMac(node, ifacename, confPath)
	return err
}

// RemoveWGQuickConf - calls wg-quick down
func RemoveWGQuickConf(confPath string, printlog bool) error {
	_, err := ncutils.RunCmd(fmt.Sprintf("wg-quick down %s", confPath), printlog)
	return err
}

// StorePrivKey - stores wg priv key on disk locally
func StorePrivKey(key string, network string) error {
	var err error
	d1 := []byte(key)
	err = os.WriteFile(ncutils.GetNetclientPathSpecific()+"wgkey-"+network, d1, 0600)
	return err
}

// RetrievePrivKey - reads wg priv key from local disk
func RetrievePrivKey(network string) (string, error) {
	dat, err := ncutils.GetFileWithRetry(ncutils.GetNetclientPathSpecific()+"wgkey-"+network, 2)
	return string(dat), err
}
