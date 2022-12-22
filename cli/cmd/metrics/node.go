package metrics

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var metricsNodeCmd = &cobra.Command{
	Use:   "node [NETWORK NAME] [NODE ID]",
	Args:  cobra.ExactArgs(2),
	Short: "Retrieve node metrics",
	Long:  `Retrieve node metrics`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.GetNodeMetrics(args[0], args[1]))
	},
}

func init() {
	rootCmd.AddCommand(metricsNodeCmd)
}
