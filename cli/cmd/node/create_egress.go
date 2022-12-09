package node

import (
	"strings"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/spf13/cobra"
)

var nodeCreateEgressCmd = &cobra.Command{
	Use:   "create_egress [NETWORK NAME] [NODE ID] [EGRESS GATEWAY ADDRESSES (comma separated)]",
	Args:  cobra.ExactArgs(3),
	Short: "Turn a Node into a Egress",
	Long:  `Turn a Node into a Egress`,
	Run: func(cmd *cobra.Command, args []string) {
		egress := &models.EgressGatewayRequest{
			NetID:     args[0],
			NodeID:    args[1],
			Interface: networkInterface,
			Ranges:    strings.Split(args[2], ","),
		}
		if natEnabled {
			egress.NatEnabled = "yes"
		}
		functions.PrettyPrint(functions.CreateEgress(args[0], args[1], egress))
	},
}

func init() {
	nodeCreateEgressCmd.Flags().StringVar(&networkInterface, "interface", "", "Network interface name (ex:- eth0)")
	nodeCreateEgressCmd.Flags().BoolVar(&natEnabled, "nat", false, "Enable NAT for Egress Traffic ?")
	rootCmd.AddCommand(nodeCreateEgressCmd)
}
