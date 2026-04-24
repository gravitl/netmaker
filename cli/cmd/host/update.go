package host

import (
	"encoding/json"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
)

var (
	apiHostFilePath string
	endpoint        string
	endpoint6       string
	name            string
	listenPort      int
	mtu             int
	isStaticPort    bool
	isStatic        bool
	isDefault       bool
	keepAlive       int
)

var hostUpdateCmd = &cobra.Command{
	Use:   "update DeviceID/HostID",
	Args:  cobra.ExactArgs(1),
	Short: "Update a device",
	Long:  `Update a device`,
	Run: func(cmd *cobra.Command, args []string) {
		apiHost := &models.ApiHost{}
		if apiHostFilePath != "" {
			content, err := os.ReadFile(apiHostFilePath)
			if err != nil {
				log.Fatal("Error when opening file: ", err)
			}
			if err := json.Unmarshal(content, apiHost); err != nil {
				log.Fatal(err)
			}
		} else {
			apiHost = functions.GetHost(args[0])
			if cmd.Flags().Changed("endpoint") {
				apiHost.EndpointIP = endpoint
			}
			if cmd.Flags().Changed("endpoint6") {
				apiHost.EndpointIPv6 = endpoint6
			}
			if cmd.Flags().Changed("name") {
				apiHost.Name = name
			}
			if cmd.Flags().Changed("listen_port") {
				apiHost.ListenPort = listenPort
			}
			if cmd.Flags().Changed("mtu") {
				apiHost.MTU = mtu
			}
			if cmd.Flags().Changed("static_port") {
				apiHost.IsStaticPort = isStaticPort
			}
			if cmd.Flags().Changed("static_endpoint") {
				apiHost.IsStatic = isStatic
			}
			if cmd.Flags().Changed("default") {
				apiHost.IsDefault = isDefault
			}
			if cmd.Flags().Changed("keep_alive") {
				apiHost.PersistentKeepalive = keepAlive
			}
		}
		functions.PrettyPrint(functions.UpdateHost(args[0], apiHost))
	},
}

func init() {
	hostUpdateCmd.Flags().StringVar(&apiHostFilePath, "file", "", "Path to host_definition.json")
	hostUpdateCmd.Flags().StringVar(&endpoint, "endpoint", "", "Endpoint of the Device")
	hostUpdateCmd.Flags().StringVar(&endpoint6, "endpoint6", "", "IPv6 Endpoint of the Device")
	hostUpdateCmd.Flags().StringVar(&name, "name", "", "Device name")
	hostUpdateCmd.Flags().IntVar(&listenPort, "listen_port", 0, "Listen port of the device")
	hostUpdateCmd.Flags().IntVar(&mtu, "mtu", 0, "Device MTU size")
	hostUpdateCmd.Flags().IntVar(&keepAlive, "keep_alive", 0, "Interval (seconds) in which packets are sent to keep connections open with peers")
	hostUpdateCmd.Flags().BoolVar(&isStaticPort, "static_port", false, "Make Device Static Port?")
	hostUpdateCmd.Flags().BoolVar(&isStatic, "static_endpoint", false, "Make Device Static Endpoint?")
	hostUpdateCmd.Flags().BoolVar(&isDefault, "default", false, "Make Device Default ?")
	rootCmd.AddCommand(hostUpdateCmd)
}
