package node

import (
	"strings"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var nodeCreateRelayCmd = &cobra.Command{
	Use:   "create_relay [NETWORK NAME] [NODE ID] [RELAY ADDRESSES (comma separated)]",
	Args:  cobra.ExactArgs(3),
	Short: "Turn a Node into a Relay",
	Long:  `Turn a Node into a Relay`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.CreateRelay(args[0], args[1], strings.Split(args[2], ",")))
	},
}

func init() {
	rootCmd.AddCommand(nodeCreateRelayCmd)
}
