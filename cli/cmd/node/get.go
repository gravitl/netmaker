package node

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var nodeGetCmd = &cobra.Command{
	Use:   "get [NETWORK NAME] [NODE ID]",
	Args:  cobra.ExactArgs(2),
	Short: "Get a node by ID",
	Long:  `Get a node by ID`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.GetNodeByID(args[0], args[1]))
	},
}

func init() {
	rootCmd.AddCommand(nodeGetCmd)
}
