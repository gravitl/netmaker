package access_token

import (
	"fmt"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var accessTokenDeleteCmd = &cobra.Command{
	Use:   "delete [ACCESS TOKEN ID]",
	Short: "Delete an access token",
	Long:  `Delete an access token by ID`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		functions.DeleteAccessToken(args[0])
		fmt.Println("Access token deleted successfully")
	},
}

func init() {
	rootCmd.AddCommand(accessTokenDeleteCmd)
}
