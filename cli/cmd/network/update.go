package network

import (
	"encoding/json"
	"log"
	"os"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/spf13/cobra"
)

var networkUpdateCmd = &cobra.Command{
	Use:   "update [NETWORK NAME]",
	Short: "Update a Network",
	Long:  `Update a Network`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var (
			networkName = args[0]
			network     = &models.Network{}
		)
		if networkDefinitionFilePath != "" {
			content, err := os.ReadFile(networkDefinitionFilePath)
			if err != nil {
				log.Fatal("Error when opening file: ", err)
			}
			if err := json.Unmarshal(content, network); err != nil {
				log.Fatal(err)
			}
		} else {
			network.NetID = networkName
			network.AddressRange = address
			if address6 != "" {
				network.AddressRange6 = address6
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
			network.DefaultInterface = defaultInterface
			network.DefaultListenPort = int32(defaultListenPort)
			network.NodeLimit = int32(nodeLimit)
			network.DefaultPostUp = defaultPostUp
			network.DefaultPostDown = defaultPostDown
			network.DefaultKeepalive = int32(defaultKeepalive)
			if allowManualSignUp {
				network.AllowManualSignUp = "yes"
			}
			network.LocalRange = localRange
			network.DefaultExtClientDNS = defaultExtClientDNS
			network.DefaultMTU = int32(defaultMTU)
		}
		functions.PrettyPrint(functions.UpdateNetwork(networkName, network))
	},
}

func init() {
	networkUpdateCmd.Flags().StringVar(&networkDefinitionFilePath, "file", "", "Path to network_definition.json")
	networkUpdateCmd.Flags().StringVar(&address, "ipv4_addr", "", "IPv4 address of the network")
	networkUpdateCmd.Flags().StringVar(&address6, "ipv6_addr", "", "IPv6 address of the network")
	networkUpdateCmd.Flags().BoolVar(&udpHolePunch, "udp_hole_punch", false, "Enable UDP Hole Punching ?")
	networkUpdateCmd.Flags().BoolVar(&localNetwork, "local", false, "Is the network local (LAN) ?")
	networkUpdateCmd.Flags().BoolVar(&defaultACL, "default_acl", false, "Enable default Access Control List ?")
	networkUpdateCmd.Flags().BoolVar(&pointToSite, "point_to_site", false, "Enforce all clients to have only 1 central peer ?")
	networkUpdateCmd.Flags().StringVar(&defaultInterface, "interface", "", "Name of the network interface")
	networkUpdateCmd.Flags().StringVar(&defaultPostUp, "post_up", "", "Commands to run after server is up `;` separated")
	networkUpdateCmd.Flags().StringVar(&defaultPostDown, "post_down", "", "Commands to run after server is down `;` separated")
	networkUpdateCmd.Flags().StringVar(&localRange, "local_range", "", "Local CIDR range")
	networkUpdateCmd.Flags().StringVar(&defaultExtClientDNS, "ext_client_dns", "", "IPv4 address of DNS server to be used by external clients")
	networkUpdateCmd.Flags().IntVar(&defaultListenPort, "listen_port", 0, "Default wireguard port each node will attempt to use")
	networkUpdateCmd.Flags().IntVar(&nodeLimit, "node_limit", 0, "Maximum number of nodes that can be associated with this network")
	networkUpdateCmd.Flags().IntVar(&defaultKeepalive, "keep_alive", 0, "Keep Alive in seconds")
	networkUpdateCmd.Flags().IntVar(&defaultMTU, "mtu", 0, "MTU size")
	networkUpdateCmd.Flags().BoolVar(&allowManualSignUp, "manual_signup", false, "Allow manual signup ?")
	rootCmd.AddCommand(networkUpdateCmd)
}
