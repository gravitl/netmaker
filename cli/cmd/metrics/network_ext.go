package metrics

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var metricsNetworkExtCmd = &cobra.Command{
	Use:   "network_ext [NETWORK NAME]",
	Args:  cobra.ExactArgs(1),
	Short: "Retrieve metrics of external clients on a given network",
	Long:  `Retrieve metrics of external clients on a given network`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.GetNetworkExtMetrics(args[0]))
	},
}

func init() {
	rootCmd.AddCommand(metricsNetworkExtCmd)
}
