package host

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var deleteHostNetworkCmd = &cobra.Command{
	Use:   "delete_network DeviceID/HostID Network",
	Args:  cobra.ExactArgs(2),
	Short: "Remove a device from a network",
	Long:  `Remove a device from a network`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.DeleteHostFromNetwork(args[0], args[1]))
	},
}

func init() {
	rootCmd.AddCommand(deleteHostNetworkCmd)
}
