package node

import (
	"fmt"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var nodeUncordonCmd = &cobra.Command{
	Use:   "uncordon [NETWORK NAME] [NODE ID]",
	Args:  cobra.ExactArgs(2),
	Short: "Get a node by ID",
	Long:  `Get a node by ID`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(*functions.UncordonNode(args[0], args[1]))
	},
}

func init() {
	rootCmd.AddCommand(nodeUncordonCmd)
}
