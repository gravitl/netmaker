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
	name            string
	listenPort      int
	mtu             int
	isStatic        bool
	isDefault       bool
	keepAlive       int
)

var hostUpdateCmd = &cobra.Command{
	Use:   "update HostID",
	Args:  cobra.ExactArgs(1),
	Short: "Update a host",
	Long:  `Update a host`,
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
			apiHost.ID = args[0]
			apiHost.EndpointIP = endpoint
			apiHost.Name = name
			apiHost.ListenPort = listenPort
			apiHost.MTU = mtu
			apiHost.IsStatic = isStatic
			apiHost.IsDefault = isDefault
			apiHost.PersistentKeepalive = keepAlive
		}
		functions.PrettyPrint(functions.UpdateHost(args[0], apiHost))
	},
}

func init() {
	hostUpdateCmd.Flags().StringVar(&apiHostFilePath, "file", "", "Path to host_definition.json")
	hostUpdateCmd.Flags().StringVar(&endpoint, "endpoint", "", "Endpoint of the Host")
	hostUpdateCmd.Flags().StringVar(&name, "name", "", "Host name")
	hostUpdateCmd.Flags().IntVar(&listenPort, "listen_port", 0, "Listen port of the host")
	hostUpdateCmd.Flags().IntVar(&mtu, "mtu", 0, "Host MTU size")
	hostUpdateCmd.Flags().IntVar(&keepAlive, "keep_alive", 0, "Interval (seconds) in which packets are sent to keep connections open with peers")
	hostUpdateCmd.Flags().BoolVar(&isStatic, "static", false, "Make Host Static ?")
	hostUpdateCmd.Flags().BoolVar(&isDefault, "default", false, "Make Host Default ?")
	rootCmd.AddCommand(hostUpdateCmd)
}
