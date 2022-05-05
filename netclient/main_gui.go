//go:build gui
// +build gui

//go:generate goversioninfo -icon=windowsdata/resource/netmaker.ico -manifest=netclient.exe.manifest.xml -64=true -o=netclient.syso
package main

import (
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/gui"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

func init() {
	config.GuiActive = true

	config.GuiRun = func() {
		networks, err := ncutils.GetSystemNetworks()
		if err != nil {
			networks = []string{}
		}
		gui.Run(networks)
	}
}
