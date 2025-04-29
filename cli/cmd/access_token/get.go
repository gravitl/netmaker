package access_token

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var accessTokenGetCmd = &cobra.Command{
	Use:   "get [USERNAME]",
	Short: "Get a user's access token",
	Long:  `Get a user's access token`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.GetAccessToken(args[0]))
	},
}

func init() {
	rootCmd.AddCommand(accessTokenGetCmd)
}
