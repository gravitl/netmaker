package wireguard

import (
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// SetWGKeyConfig - sets the wg conf with a new private key
func SetWGKeyConfig(network string, serveraddr string) error {

	cfg, err := config.ReadConfig(network)
	if err != nil {
		return err
	}

	node := cfg.Node

	privatekey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return err
	}
	privkeystring := privatekey.String()
	publickey := privatekey.PublicKey()

	node.PublicKey = publickey.String()

	err = StorePrivKey(privkeystring, network)
	if err != nil {
		return err
	}
	if node.Action == models.NODE_UPDATE_KEY {
		node.Action = models.NODE_NOOP
	}
	err = config.ModConfig(&node)
	if err != nil {
		return err
	}

	err = SetWGConfig(network, false)
	if err != nil {
		return err
	}

	return err
}

// ApplyWGQuickConf - applies wg-quick commands if os supports
func ApplyWGQuickConf(confPath string) error {
	_, err := os.Stat(confPath)
	if err != nil {
		ncutils.Log(confPath + " does not exist " + err.Error())
		return err
	}
	_, err = ncutils.RunCmd("wg-quick down "+confPath, false)
	if err != nil {
		ncutils.Log("err runing wg-quick down " + confPath + err.Error())
	}
	_, err = ncutils.RunCmd("wg-quick up "+confPath, false)
	if err != nil {
		ncutils.Log("err runing wg-quick up " + confPath + err.Error())
	}
	return err
}

// SyncWGQuickConf - formats config file and runs sync command
func SyncWGQuickConf(iface string, confPath string) error {
	var tmpConf = confPath + ".sync.tmp"
	confRaw, err := ncutils.RunCmd("wg-quick strip "+confPath, false)
	if err != nil {
		return err
	}
	regex := regexp.MustCompile(".*Warning.*\n")
	conf := regex.ReplaceAllString(confRaw, "")
	err = os.WriteFile(tmpConf, []byte(conf), 0644)
	if err != nil {
		return err
	}
	_, err = ncutils.RunCmd("wg syncconf "+iface+" "+tmpConf, true)
	if err != nil {
		log.Println(err.Error())
		ncutils.Log("error syncing conf, resetting")
		err = ApplyWGQuickConf(confPath)
	}
	errN := os.Remove(tmpConf)
	if errN != nil {
		ncutils.Log(errN.Error())
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
	err = os.WriteFile(ncutils.GetNetclientPathSpecific()+"wgkey-"+network, d1, 0644)
	return err
}

// RetrievePrivKey - reads wg priv key from local disk
func RetrievePrivKey(network string) (string, error) {
	dat, err := os.ReadFile(ncutils.GetNetclientPathSpecific() + "wgkey-" + network)
	return string(dat), err
}
