//go:build gui
// +build gui

package main

import (
	"github.com/netmakerio/netmaker/netclient/config"
	"github.com/netmakerio/netmaker/netclient/gui"
	"github.com/netmakerio/netmaker/netclient/ncutils"
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
