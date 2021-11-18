package wireguard

import (
	"fmt"

	"github.com/gravitl/netmaker/netclient/ncutils"
)

func ApplyWindowsConf(confPath string) error {
	var commandLine = fmt.Sprintf(`wireguard.exe /installtunnelservice "%s"`, confPath)
	if _, err := ncutils.RunCmdFormatted(commandLine, false); err != nil {
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
