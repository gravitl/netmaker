package failover

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var disableFailoverCmd = &cobra.Command{
	Use:   "disable [NODE ID]",
	Args:  cobra.ExactArgs(1),
	Short: "Disable failover for a given Node",
	Long:  `Disable failover for a given Node`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.DisableNodeFailover(args[0]))
	},
}

func init() {
	rootCmd.AddCommand(disableFailoverCmd)
}
