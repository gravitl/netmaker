package ext_client

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var extClientDeleteCmd = &cobra.Command{
	Use:   "delete [NETWORK NAME] [EXTERNAL CLIENT ID]",
	Args:  cobra.ExactArgs(2),
	Short: "Delete an External Client",
	Long:  `Delete an External Client`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.DeleteExtClient(args[0], args[1]))
	},
}

func init() {
	rootCmd.AddCommand(extClientDeleteCmd)
}
