package node

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var nodeDeleteIngressCmd = &cobra.Command{
	Use:        "delete_remote_access_gateway [NETWORK NAME] [NODE ID]",
	Args:       cobra.ExactArgs(2),
	Short:      "Delete Remote Access Gateway role from a Node",
	Long:       `Delete Remote Access Gateway role from a Node`,
	Deprecated: "in favour of the `gateway` subcommand, in Netmaker v0.90.0.",
	Aliases:    []string{"delete_rag"},
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.DeleteIngress(args[0], args[1]))
	},
}

func init() {
	rootCmd.AddCommand(nodeDeleteIngressCmd)
}
