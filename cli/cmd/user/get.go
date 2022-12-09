package user

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var userGetCmd = &cobra.Command{
	Use:   "get [USER NAME]",
	Args:  cobra.ExactArgs(1),
	Short: "Get a user",
	Long:  `Get a user`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.GetUser(args[0]))
	},
}

func init() {
	rootCmd.AddCommand(userGetCmd)
}
