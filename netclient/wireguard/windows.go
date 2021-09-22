package wireguard

import "github.com/gravitl/netmaker/netclient/ncutils"

func ApplyWindowsConf(confPath string) error {
	if _, err := ncutils.RunCmd("wireguard.exe /installtunnelservice "+confPath, false); err != nil {
		return err
	}
	return nil
}

func RemoveWindowsConf(ifacename string, printlog bool) error {
	if _, err := ncutils.RunCmd("wireguard.exe /uninstalltunnelservice "+ifacename, printlog); err != nil {
		return err
	}
	return nil
}
