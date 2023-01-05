package node

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var nodeDeleteRelayCmd = &cobra.Command{
	Use:   "delete_relay [NETWORK NAME] [NODE ID]",
	Args:  cobra.ExactArgs(2),
	Short: "Delete Relay role from a Node",
	Long:  `Delete Relay role from a Node`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.DeleteRelay(args[0], args[1]))
	},
}

func init() {
	rootCmd.AddCommand(nodeDeleteRelayCmd)
}
