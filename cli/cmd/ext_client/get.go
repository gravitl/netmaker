package ext_client

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var extClientGetCmd = &cobra.Command{
	Use:   "get [NETWORK NAME] [EXTERNAL CLIENT ID]",
	Args:  cobra.ExactArgs(2),
	Short: "Get an External Client",
	Long:  `Get an External Client`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.GetExtClient(args[0], args[1]))
	},
}

func init() {
	rootCmd.AddCommand(extClientGetCmd)
}
