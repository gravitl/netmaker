package wireguard

import (
	"io/ioutil"

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
	if _, err := ncutils.RunCmd("wg-quick up "+confPath, true); err != nil {
		return err
	}
	return nil
}

// RemoveWGQuickConf - calls wg-quick down
func RemoveWGQuickConf(confPath string, printlog bool) error {
	if _, err := ncutils.RunCmd("wg-quick down "+confPath, printlog); err != nil {
		return err
	}
	return nil
}

// StorePrivKey - stores wg priv key on disk locally
func StorePrivKey(key string, network string) error {
	var err error
	d1 := []byte(key)
	err = ioutil.WriteFile(ncutils.GetNetclientPathSpecific()+"wgkey-"+network, d1, 0644)
	return err
}

// RetrievePrivKey - reads wg priv key from local disk
func RetrievePrivKey(network string) (string, error) {
	dat, err := ioutil.ReadFile(ncutils.GetNetclientPathSpecific() + "wgkey-" + network)
	return string(dat), err
}
