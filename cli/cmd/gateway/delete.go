package gateway

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var gatewayDeleteCmd = &cobra.Command{
	Use:   "delete [NETWORK NAME] [NODE ID]",
	Args:  cobra.ExactArgs(2),
	Short: "Delete a Gateway.",
	Long: `
Removes the gateway configuration from a node in a specified network. The node itself remains, but it will no longer function as a gateway.

Arguments:
NETWORK NAME:	The name of the network from which the gateway configuration should be removed.
NODE ID:		The ID of the node that is currently acting as a gateway.
`,
	Aliases: []string{"rm"},
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.DeleteGateway(args[0], args[1]))
	},
}

func init() {
	rootCmd.AddCommand(gatewayDeleteCmd)
}
