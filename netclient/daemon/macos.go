package daemon

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

const MAC_SERVICE_NAME = "com.gravitl.netclient"

func SetupMacDaemon() error {

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return err
	}
	binarypath := dir + "/netclient"

	if !ncutils.FileExists("/etc/netclient/netclient") {
		_, err = ncutils.Copy(binarypath, "/etc/netclient/netclient")
		if err != nil {
			log.Println(err)
			return err
		}
	}

	_, errN := os.Stat("~/Library/LaunchAgents")
	if os.IsNotExist(errN) {
		os.Mkdir("~/Library/LaunchAgents", 0755)
	}
	err = CreateMacService(MAC_SERVICE_NAME)
	if err != nil {
		return err
	}
	_, err = ncutils.RunCmd("launchctl load /Library/LaunchDaemons/"+MAC_SERVICE_NAME+".plist", true)
	return err
}

func CleanupMac() {
	_, err := ncutils.RunCmd("launchctl unload /Library/LaunchDaemons/"+MAC_SERVICE_NAME+".plist", true)
	if ncutils.FileExists("/Library/LaunchDaemons/" + MAC_SERVICE_NAME + ".plist") {
		err = os.Remove("/Library/LaunchDaemons/" + MAC_SERVICE_NAME + ".plist")
	}
	if err != nil {
		ncutils.PrintLog(err.Error(), 1)
	}

	os.RemoveAll(ncutils.GetNetclientPath())
}

func CreateMacService(servicename string) error {
	_, err := os.Stat("/Library/LaunchDaemons")
	if os.IsNotExist(err) {
		os.Mkdir("/Library/LaunchDaemons", 0755)
	} else if err != nil {
		log.Println("couldnt find or create /Library/LaunchDaemons")
		return err
	}
	daemonstring := MacDaemonString()
	daemonbytes := []byte(daemonstring)

	if !ncutils.FileExists("/Library/LaunchDaemons/com.gravitl.netclient.plist") {
		err = ioutil.WriteFile("/Library/LaunchDaemons/com.gravitl.netclient.plist", daemonbytes, 0644)
	}
	return err
}

func MacDaemonString() string {
	return `<?xml version='1.0' encoding='UTF-8'?>
<!DOCTYPE plist PUBLIC \"-//Apple Computer//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\" >
<plist version='1.0'>
<dict>
	<key>Label</key><string>com.gravitl.netclient</string>
	<key>ProgramArguments</key>
		<array>
			<string>/etc/netclient/netclient</string>
			<string>checkin</string>
			<string>-n</string>
			<string>all</string>
		</array>
	<key>StandardOutPath</key><string>/etc/netclient/com.gravitl.netclient.log</string>
	<key>StandardErrorPath</key><string>/etc/netclient/com.gravitl.netclient.log</string>
	<key>AbandonProcessGroup</key><true/>
	<key>StartInterval</key>
	    <integer>15</integer>
	<key>EnvironmentVariables</key>
		<dict>
			<key>PATH</key>
			<string>/usr/local/sbin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
		</dict>
</dict>
</plist>
`
}

type MacTemplateData struct {
	Label    string
	Interval string
}
