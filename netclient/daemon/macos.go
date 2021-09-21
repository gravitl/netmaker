package daemon

import (
	"fmt"
	"log"
	"os"
	"text/template"

	"github.com/gravitl/netmaker/netclient/ncutils"
)

const MAC_SERVICE_NAME = "com.gravitl.netclient"

func SetupMacDaemon() error {
	_, errN := os.Stat("~/Library/LaunchAgents")
	if os.IsNotExist(errN) {
		os.Mkdir("~/Library/LaunchAgents", 0755)
	}
	err := CreateMacService(MAC_SERVICE_NAME)
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
	tdata := MacTemplateData{
		Label:    servicename,
		Interval: "15",
	}
	_, err := os.Stat("/Library/LaunchDaemons")
	if os.IsNotExist(err) {
		os.Mkdir("/Library/LaunchDaemons", 0755)
	} else if err != nil {
		log.Println("couldnt find or create /Library/LaunchDaemons")
		return err
	}
	fileLoc := fmt.Sprintf("/Library/LaunchDaemons/%s.plist", tdata.Label)
	launchdFile, err := os.Open(fileLoc)
	if err != nil {
		return err
	}
	launchdTemplate := template.Must(template.New("launchdTemplate").Parse(MacTemplate()))
	return launchdTemplate.Execute(launchdFile, tdata)
}

func MacTemplate() string {
	return `
	<?xml version='1.0' encoding='UTF-8'?>
	<!DOCTYPE plist PUBLIC \"-//Apple Computer//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\" >
	<plist version='1.0'>
	  <dict>
		<key>Label</key><string>{{.Label}}</string>
		<key>ProgramArguments</key>
	   <array>
		   <string>/etc/netclient/netclient</string>
		   <string>checkin</string>
		   <string>-n</string>
		   <string>all</string>
	   </array>
		<key>StandardOutPath</key><string>/etc/netclient/{{.Label}}.log</string>
		<key>StandardErrorPath</key><string>/etc/netclient/{{.Label}}.log</string>
		<key>AbandonProcessGroup</key><true/>
		<key>StartInterval</key>
	   <integer>{{.Interval}}</integer>
		<key>EnvironmentVariables</key>
		   <dict>
			   <key>PATH</key>
			   <string>/usr/local/sbin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
		   </dict>
	 </dict>
   </plist>
</plist>
`
}

type MacTemplateData struct {
	Label    string
	Interval string
}
