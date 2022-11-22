package node

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var nodeDeleteEgressCmd = &cobra.Command{
	Use:   "delete_egress [NETWORK NAME] [NODE ID]",
	Args:  cobra.ExactArgs(2),
	Short: "Delete Egress role from a Node",
	Long:  `Delete Egress role from a Node`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.DeleteEgress(args[0], args[1]))
	},
}

func init() {
	rootCmd.AddCommand(nodeDeleteEgressCmd)
}
