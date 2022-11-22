package network

import (
	"encoding/json"
	"log"
	"os"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/spf13/cobra"
)

var (
	networkDefinitionFilePath string
	netID                     string
	ipv4Address               string
	ipv6Address               string
	udpHolePunch              bool
	localNetwork              bool
	defaultACL                bool
	pointToSite               bool
)

// networkCreateCmd represents the networkCreate command
var networkCreateCmd = &cobra.Command{
	Use:   "create [--flags]",
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
			network.AddressRange = ipv4Address
			if ipv6Address != "" {
				network.AddressRange6 = ipv6Address
				network.IsIPv6 = "yes"
			}
			if udpHolePunch {
				network.DefaultUDPHolePunch = "yes"
			}
			if localNetwork {
				network.IsLocal = "yes"
			}
			if defaultACL {
				network.DefaultACL = "yes"
			}
			if pointToSite {
				network.IsPointToSite = "yes"
			}
		}
		functions.PrettyPrint(functions.CreateNetwork(network))
	},
}

func init() {
	networkCreateCmd.Flags().StringVar(&networkDefinitionFilePath, "file", "", "Path to network_definition.json")
	networkCreateCmd.Flags().StringVar(&netID, "name", "", "Name of the network")
	networkCreateCmd.MarkFlagsMutuallyExclusive("file", "name")

	networkCreateCmd.Flags().StringVar(&ipv4Address, "ipv4_addr", "", "IPv4 address of the network")
	networkCreateCmd.Flags().StringVar(&ipv6Address, "ipv6_addr", "", "IPv6 address of the network")
	networkCreateCmd.Flags().BoolVar(&udpHolePunch, "udp_hole_punch", false, "Enable UDP Hole Punching ?")
	networkCreateCmd.Flags().BoolVar(&localNetwork, "local", false, "Is the network local (LAN) ?")
	networkCreateCmd.Flags().BoolVar(&defaultACL, "default_acl", true, "Enable default Access Control List ?")
	networkCreateCmd.Flags().BoolVar(&pointToSite, "point_to_site", false, "Enforce all clients to have only 1 central peer ?")
	rootCmd.AddCommand(networkCreateCmd)
}
