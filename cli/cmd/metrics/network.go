package metrics

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var metricsNetworkCmd = &cobra.Command{
	Use:   "network [NETWORK NAME]",
	Args:  cobra.ExactArgs(1),
	Short: "Retrieve network metrics",
	Long:  `Retrieve network metrics`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.GetNetworkNodeMetrics(args[0]))
	},
}

func init() {
	rootCmd.AddCommand(metricsNetworkCmd)
}
