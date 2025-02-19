package node

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var hostDeleteRelayCmd = &cobra.Command{
	Use:        "delete_relay [NETWORK] [NODE ID]",
	Args:       cobra.ExactArgs(2),
	Short:      "Delete Relay from a node",
	Long:       `Delete Relay from a node`,
	Deprecated: "in favour of the `gateway` subcommand.",
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.DeleteRelay(args[0], args[1]))
	},
}

func init() {
	rootCmd.AddCommand(hostDeleteRelayCmd)
}
