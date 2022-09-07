package wireguard

import (
	"fmt"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// ApplyWindowsConf - applies the WireGuard configuration file on Windows
func ApplyWindowsConf(confPath string, isConnected bool) error {
	if !isConnected {
		return nil
	}
	var commandLine = fmt.Sprintf(`wireguard.exe /installtunnelservice "%s"`, confPath)
	if _, err := ncutils.RunCmdFormatted(commandLine, false); err != nil {
		return err
	}
	return nil
}

// RemoveWindowsConf - removes the WireGuard configuration file on Windows and dpapi file
func RemoveWindowsConf(ifacename string, printlog bool) error {
	if _, err := ncutils.RunCmd("wireguard.exe /uninstalltunnelservice "+ifacename, printlog); err != nil {
		logger.Log(1, err.Error())
	}
	return nil
}
