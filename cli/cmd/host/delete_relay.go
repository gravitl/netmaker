package host

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var hostDeleteRelayCmd = &cobra.Command{
	Use:   "delete_relay [HOST ID]",
	Args:  cobra.ExactArgs(1),
	Short: "Delete Relay role from a host",
	Long:  `Delete Relay role from a host`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.DeleteRelay(args[0]))
	},
}

func init() {
	rootCmd.AddCommand(hostDeleteRelayCmd)
}
