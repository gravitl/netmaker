package daemon

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gravitl/netmaker/netclient/ncutils"
)

const MAC_SERVICE_NAME = "com.gravitl.netclient"
const EXEC_DIR = "/sbin/"

// SetupMacDaemon - Creates a daemon service from the netclient under LaunchAgents for MacOS
func SetupMacDaemon(interval string) error {

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return err
	}
	binarypath := dir + "/netclient"

	if !ncutils.FileExists(EXEC_DIR + "netclient") {
		err = ncutils.Copy(binarypath, EXEC_DIR+"netclient")
		if err != nil {
			log.Println(err)
			return err
		}
	}

	_, errN := os.Stat("~/Library/LaunchAgents")
	if os.IsNotExist(errN) {
		os.Mkdir("~/Library/LaunchAgents", 0755)
	}
	err = CreateMacService(MAC_SERVICE_NAME, interval)
	if err != nil {
		return err
	}
	_, err = ncutils.RunCmd("launchctl load /Library/LaunchDaemons/"+MAC_SERVICE_NAME+".plist", true)
	return err
}

// CleanupMac - Removes the netclient checkin daemon from LaunchDaemons
func CleanupMac() {
	_, err := ncutils.RunCmd("launchctl unload /Library/LaunchDaemons/"+MAC_SERVICE_NAME+".plist", true)
	if ncutils.FileExists("/Library/LaunchDaemons/" + MAC_SERVICE_NAME + ".plist") {
		err = os.Remove("/Library/LaunchDaemons/" + MAC_SERVICE_NAME + ".plist")
	}
	if err != nil {
		ncutils.PrintLog(err.Error(), 1)
	}

	os.RemoveAll(ncutils.GetNetclientPath())
	os.Remove(EXEC_DIR + "netclient")
}

// CreateMacService - Creates the mac service file for LaunchDaemons
func CreateMacService(servicename string, interval string) error {
	_, err := os.Stat("/Library/LaunchDaemons")
	if os.IsNotExist(err) {
		os.Mkdir("/Library/LaunchDaemons", 0755)
	} else if err != nil {
		log.Println("couldnt find or create /Library/LaunchDaemons")
		return err
	}
	daemonstring := MacDaemonString(interval)
	daemonbytes := []byte(daemonstring)

	if !ncutils.FileExists("/Library/LaunchDaemons/com.gravitl.netclient.plist") {
		err = os.WriteFile("/Library/LaunchDaemons/com.gravitl.netclient.plist", daemonbytes, 0644)
	}
	return err
}

// MacDaemonString - the file contents for the mac netclient daemon service (launchdaemon)
func MacDaemonString(interval string) string {
	return fmt.Sprintf(`<?xml version='1.0' encoding='UTF-8'?>
<!DOCTYPE plist PUBLIC \"-//Apple Computer//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\" >
<plist version='1.0'>
<dict>
	<key>Label</key><string>com.gravitl.netclient</string>
	<key>ProgramArguments</key>
		<array>
			<string>/sbin/netclient</string>
			<string>daemon</string>
		</array>
	<key>StandardOutPath</key><string>/etc/netclient/com.gravitl.netclient.log</string>
	<key>StandardErrorPath</key><string>/etc/netclient/com.gravitl.netclient.log</string>
	<key>AbandonProcessGroup</key><true/>
	<key>StartInterval</key>
	    <integer>%s</integer>
	<key>EnvironmentVariables</key>
		<dict>
			<key>PATH</key>
			<string>/usr/local/sbin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
		</dict>
</dict>
</plist>
`, interval)
}

// MacTemplateData - struct to represent the mac service
type MacTemplateData struct {
	Label    string
	Interval string
}
