package network

import (
	"encoding/json"
	"log"
	"os"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/spf13/cobra"
)

var networkCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a Network",
	Long:  `Create a Network`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		network := &models.Network{}
		if networkDefinitionFilePath != "" {
			content, err := os.ReadFile(networkDefinitionFilePath)
			if err != nil {
				log.Fatal("Error when opening file: ", err)
			}
			if err := json.Unmarshal(content, network); err != nil {
				log.Fatal(err)
			}
		} else {
			network.NetID = netID
			network.AddressRange = address
			if address6 != "" {
				network.AddressRange6 = address6
				network.IsIPv6 = "yes"
			}
			if address == "" {
				network.IsIPv4 = "no"
			}
			if defaultACL {
				network.DefaultACL = "yes"
			}
			network.DefaultKeepalive = int32(defaultKeepalive)
			network.DefaultMTU = int32(defaultMTU)
		}
		functions.PrettyPrint(functions.CreateNetwork(network))
	},
}

func init() {
	networkCreateCmd.Flags().StringVar(&networkDefinitionFilePath, "file", "", "Path to network_definition.json")
	networkCreateCmd.Flags().StringVar(&netID, "name", "", "Name of the network")
	networkCreateCmd.MarkFlagsMutuallyExclusive("file", "name")
	networkCreateCmd.Flags().StringVar(&address, "ipv4_addr", "", "IPv4 address of the network")
	networkCreateCmd.Flags().StringVar(&address6, "ipv6_addr", "", "IPv6 address of the network")
	networkCreateCmd.Flags().BoolVar(&defaultACL, "default_acl", false, "Enable default Access Control List ?")
	networkCreateCmd.Flags().IntVar(&defaultKeepalive, "keep_alive", 20, "Keep Alive in seconds")
	networkCreateCmd.Flags().IntVar(&defaultMTU, "mtu", 1280, "MTU size")
	rootCmd.AddCommand(networkCreateCmd)
}
