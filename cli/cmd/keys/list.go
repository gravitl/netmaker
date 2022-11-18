package keys

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var keysListCmd = &cobra.Command{
	Use:   "list [NETWORK NAME]",
	Args:  cobra.ExactArgs(1),
	Short: "List all keys associated with a network",
	Long:  `List all keys associated with a network`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.GetKeys(args[0]))
	},
}

func init() {
	rootCmd.AddCommand(keysListCmd)
}
