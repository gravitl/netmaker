package keys

import (
	"fmt"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var keysDeleteCmd = &cobra.Command{
	Use:   "delete [NETWORK NAME] [KEY NAME]",
	Args:  cobra.ExactArgs(2),
	Short: "Delete a key",
	Long:  `Delete a key`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.DeleteKey(args[0], args[1])
		fmt.Println("Success")
	},
}

func init() {
	rootCmd.AddCommand(keysDeleteCmd)
}
