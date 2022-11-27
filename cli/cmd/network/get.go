package network

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var networkGetCmd = &cobra.Command{
	Use:   "get [NETWORK NAME]",
	Short: "Get a Network",
	Long:  `Get a Network`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.GetNetwork(args[0]))
	},
}

func init() {
	rootCmd.AddCommand(networkGetCmd)
}
