package user

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var userListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.NoArgs,
	Short: "List all users",
	Long:  `List all users`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.ListUsers())
	},
}

func init() {
	rootCmd.AddCommand(userListCmd)
}
