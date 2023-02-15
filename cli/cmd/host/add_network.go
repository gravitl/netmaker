package host

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var addHostNetworkCmd = &cobra.Command{
	Use:   "add_network HostID Network",
	Args:  cobra.ExactArgs(2),
	Short: "Add a network to a host",
	Long:  `Add a network to a host`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.AddHostToNetwork(args[0], args[1]))
	},
}

func init() {
	rootCmd.AddCommand(addHostNetworkCmd)
}
