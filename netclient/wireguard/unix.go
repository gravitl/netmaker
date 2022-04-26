package wireguard

import (
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// ApplyWGQuickConf - applies wg-quick commands if os supports
func ApplyWGQuickConf(confPath string, ifacename string) error {
	if ncutils.IsWindows() {
		return ApplyWindowsConf(confPath)
	} else {
		_, err := os.Stat(confPath)
		if err != nil {
			logger.Log(0, confPath+" does not exist "+err.Error())
			return err
		}
		if ncutils.IfaceExists(ifacename) {
			ncutils.RunCmd("wg-quick down "+confPath, true)
		}
		_, err = ncutils.RunCmd("wg-quick up "+confPath, true)

		return err
	}
}

// ApplyMacOSConf - applies system commands similar to wg-quick using golang for MacOS
func ApplyMacOSConf(node *models.Node, ifacename string, confPath string) error {
	var err error
	_ = WgQuickDownMac(node, ifacename)
	err = WgQuickUpMac(node, ifacename, confPath)
	return err
}

// SyncWGQuickConf - formats config file and runs sync command
func SyncWGQuickConf(iface string, confPath string) error {
	var tmpConf = confPath + ".sync.tmp"
	var confCmd = "wg-quick strip "
	if ncutils.IsMac() {
		confCmd = "grep -v -e Address -e MTU -e PostUp -e PostDown "
	}
	confRaw, err := ncutils.RunCmd(confCmd+confPath, false)
	if err != nil {
		return err
	}
	regex := regexp.MustCompile(".*Warning.*\n")
	conf := regex.ReplaceAllString(confRaw, "")
	err = os.WriteFile(tmpConf, []byte(conf), 0600)
	if err != nil {
		return err
	}
	_, err = ncutils.RunCmd("wg syncconf "+iface+" "+tmpConf, true)
	if err != nil {
		log.Println(err.Error())
		logger.Log(0, "error syncing conf, resetting")
		err = ApplyWGQuickConf(confPath, iface)
	}
	errN := os.Remove(tmpConf)
	if errN != nil {
		logger.Log(0, errN.Error())
	}
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
