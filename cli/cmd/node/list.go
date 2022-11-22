package node

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var networkName string

// nodeListCmd lists all nodes
var nodeListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.NoArgs,
	Short: "List all nodes",
	Long:  `List all nodes`,
	Run: func(cmd *cobra.Command, args []string) {
		if networkName != "" {
			functions.PrettyPrint(functions.GetNodes(networkName))
		} else {
			functions.PrettyPrint(functions.GetNodes())
		}
	},
}

func init() {
	nodeListCmd.Flags().StringVar(&networkName, "network", "", "Network name specifier")
	rootCmd.AddCommand(nodeListCmd)
}
