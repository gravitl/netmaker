package host

import (
	"strings"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var hostCreateRelayCmd = &cobra.Command{
	Use:   "create_relay [HOST ID] [RELAYED HOST IDS (comma separated)]",
	Args:  cobra.ExactArgs(2),
	Short: "Turn a Host into a Relay",
	Long:  `Turn a Host into a Relay`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.CreateRelay(args[0], strings.Split(args[1], ",")))
	},
}

func init() {
	rootCmd.AddCommand(hostCreateRelayCmd)
}
