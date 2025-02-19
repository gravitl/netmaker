package node

import (
	"strings"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var hostCreateRelayCmd = &cobra.Command{
	Use:        "create_relay [NETWORK][NODE ID] [RELAYED NODE IDS (comma separated)]",
	Args:       cobra.ExactArgs(3),
	Short:      "Turn a Node into a Relay",
	Long:       `Turn a Node into a Relay`,
	Deprecated: "in favour of the `gateway` subcommand.",
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.CreateRelay(args[0], args[1], strings.Split(args[2], ",")))
	},
}

func init() {
	rootCmd.AddCommand(hostCreateRelayCmd)
}
