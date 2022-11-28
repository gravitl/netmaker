package metrics

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var metricsAllCmd = &cobra.Command{
	Use:   "all",
	Args:  cobra.NoArgs,
	Short: "Retrieve all metrics",
	Long:  `Retrieve all metrics`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.GetAllMetrics())
	},
}

func init() {
	rootCmd.AddCommand(metricsAllCmd)
}
