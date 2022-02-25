package wireguard

import (
	"fmt"

	"github.com/gravitl/netmaker/netclient/ncutils"
)

// ApplyWindowsConf - applies the WireGuard configuration file on Windows
func ApplyWindowsConf(confPath string) error {
	/*
		pathStrings := strings.Split(confPath, ncutils.GetWGPathSpecific())
		if len(pathStrings) == 2 {
			copyConfPath := fmt.Sprintf("%s\\%s", ncutils.WINDOWS_WG_DPAPI_PATH, pathStrings[1])
			err := ncutils.Copy(confPath, copyConfPath)
			if err != nil {
				ncutils.PrintLog(err.Error(), 1)
			}
		}
	*/
	var commandLine = fmt.Sprintf(`wireguard.exe /installtunnelservice "%s"`, confPath)
	if _, err := ncutils.RunCmdFormatted(commandLine, false); err != nil {
		return err
	}
	return nil
}

// RemoveWindowsConf - removes the WireGuard configuration file on Windows and dpapi file
func RemoveWindowsConf(ifacename string, printlog bool) error {
	if _, err := ncutils.RunCmd("wireguard.exe /uninstalltunnelservice "+ifacename, printlog); err != nil {
		ncutils.PrintLog(err.Error(), 1)
	}
	/*
		dpapipath := fmt.Sprintf("%s\\%s.conf.dpapi", ncutils.WINDOWS_WG_DPAPI_PATH, ifacename)
		confpath := fmt.Sprintf("%s\\%s.conf", ncutils.WINDOWS_WG_DPAPI_PATH, ifacename)
		if ncutils.FileExists(confpath) {
			err := os.Remove(confpath)
			if err != nil {
				ncutils.PrintLog(err.Error(), 1)
			}
		}
		time.Sleep(time.Second >> 2)
		if ncutils.FileExists(dpapipath) {
			err := os.Remove(dpapipath)
			if err != nil {
				ncutils.PrintLog(err.Error(), 1)
			}
		}
	*/
	return nil
}
