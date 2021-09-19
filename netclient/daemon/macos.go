package daemon

import (
	"fmt"
	"log"
	"os"
	"text/template"

	"github.com/gravitl/netmaker/netclient/ncutils"
)

const MAC_SERVICE_NAME = "com.gravitl.netclient"

func CreateAndRunMacDaemon() error {
	_, err := os.Stat("~/Library/LaunchAgents")
	if os.IsNotExist(err) {
		os.Mkdir("~/Library/LaunchAgents", 0744)
	}
	err = CreateMacService(MAC_SERVICE_NAME)
	if err != nil {
		return err
	}
	_, err = ncutils.RunCmd("launchctl load ~/Library/LaunchAgents/"+MAC_SERVICE_NAME+".plist", true)
	return err
}

func CleanupMac() {
	//StopWindowsDaemon()
	//RemoveWindowsDaemon()
	//os.RemoveAll(ncutils.GetNetclientPath())
	log.Println("TODO: Not implemented yet")
}

func CreateMacService(servicename string) error {
	tdata := MacTemplateData{
		Label:     servicename,
		Program:   "/etc/netclient/netclient",
		KeepAlive: true,
		RunAtLoad: true,
	}
	fileLoc := fmt.Sprintf("%s/Library/LaunchAgents/%s.plist", os.Getenv("HOME"), tdata.Label)
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
     <key>Program</key><string>{{.Program}}</string>
     <key>StandardOutPath</key><string>/tmp/{{.Label}}.out.log</string>
     <key>StandardErrorPath</key><string>/tmp/{{.Label}}.err.log</string>
     <key>KeepAlive</key><{{.KeepAlive}}/>
     <key>RunAtLoad</key><{{.RunAtLoad}}/>
	 <key>StartCalendarInterval</key>
	 <dict>
	 	<key>Minute</key>
	 	<value>*/1</value>
   	 </dict>
</plist>
`
}

type MacTemplateData struct {
	Label     string
	Program   string
	KeepAlive bool
	RunAtLoad bool
}
