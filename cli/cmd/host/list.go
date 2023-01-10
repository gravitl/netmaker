package host

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var hostListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.NoArgs,
	Short: "List all hosts",
	Long:  `List all hosts`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.GetHosts())
	},
}

func init() {
	rootCmd.AddCommand(hostListCmd)
}
