package node

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var nodeDeleteIngressCmd = &cobra.Command{
	Use:   "delete_ingress [NETWORK NAME] [NODE ID]",
	Args:  cobra.ExactArgs(2),
	Short: "Delete Ingress role from a Node",
	Long:  `Delete Ingress role from a Node`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.DeleteIngress(args[0], args[1]))
	},
}

func init() {
	rootCmd.AddCommand(nodeDeleteIngressCmd)
}
