//go:generate goversioninfo -icon=windowsdata/resource/netclient.ico -manifest=netclient.exe.manifest.xml -64=true -o=netclient.syso
// -build gui

package main

import (
	"log"
	"os"
	"runtime/debug"

	"github.com/gravitl/netmaker/netclient/cli_options"
	"github.com/gravitl/netmaker/netclient/config"
	"github.com/gravitl/netmaker/netclient/functions"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/gravitl/netmaker/netclient/ncwindows"
	"github.com/urfave/cli/v2"
)

var version = "dev"

func main() {
	app := cli.NewApp()
	app.Name = "Netclient"
	app.Version = version
	ncutils.SetVersion(version)
	cliFlags := cli_options.GetFlags(ncutils.GetHostname())
	app.Commands = cli_options.GetCommands(cliFlags[:])
	app.Description = "Used to perform interactions with Netmaker server and set local WireGuard config."
	app.Usage = "Netmaker's netclient agent and CLI."
	app.UsageText = "netclient [global options] command [command options] [arguments...]. Adjust verbosity of given command with -v, -vv or -vvv (max)."

	setGarbageCollection()
	functions.SetHTTPClient()

	if ncutils.IsWindows() {
		ncwindows.InitWindows()
	} else {
		ncutils.CheckUID()
		ncutils.CheckWG()
		if ncutils.IsLinux() {
			ncutils.CheckFirewall()
		}
	}

	if len(os.Args) <= 1 && config.GuiActive {
		config.GuiRun.(func())()
	} else {
		err := app.Run(os.Args)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func setGarbageCollection() {
	_, gcset := os.LookupEnv("GOGC")
	if !gcset {
		debug.SetGCPercent(ncutils.DEFAULT_GC_PERCENT)
	}
}
