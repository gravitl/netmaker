package node

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var nodeCreateIngressCmd = &cobra.Command{
	Use:        "create_remote_access_gateway [NETWORK NAME] [NODE ID]",
	Args:       cobra.ExactArgs(2),
	Short:      "Turn a Node into a Remote Access Gateway (Ingress)",
	Long:       `Turn a Node into a Remote Access Gateway (Ingress) for a Network.`,
	Deprecated: "in favour of the `gateway` subcommand.",
	Aliases:    []string{"create_rag"},
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.CreateIngress(args[0], args[1], failover))
	},
}

func init() {
	nodeCreateIngressCmd.Flags().BoolVar(&failover, "failover", false, "Enable FailOver ?")
	rootCmd.AddCommand(nodeCreateIngressCmd)
}
