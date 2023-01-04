package user

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var userDeleteCmd = &cobra.Command{
	Use:   "delete [USER NAME]",
	Args:  cobra.ExactArgs(1),
	Short: "Delete a user",
	Long:  `Delete a user`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(*functions.DeleteUser(args[0]))
	},
}

func init() {
	rootCmd.AddCommand(userDeleteCmd)
}
