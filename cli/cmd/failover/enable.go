package failover

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var enableFailoverCmd = &cobra.Command{
	Use:   "enable [NODE ID]",
	Args:  cobra.ExactArgs(1),
	Short: "Enable failover for a given Node",
	Long:  `Enable failover for a given Node`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.EnableNodeFailover(args[0]))
	},
}

func init() {
	rootCmd.AddCommand(enableFailoverCmd)
}
