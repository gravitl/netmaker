package usergroup

import (
	"fmt"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var usergroupDeleteCmd = &cobra.Command{
	Use:   "delete [GROUP NAME]",
	Args:  cobra.ExactArgs(1),
	Short: "Delete a usergroup",
	Long:  `Delete a usergroup`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.DeleteUsergroup(args[0])
		fmt.Println("Success")
	},
}

func init() {
	rootCmd.AddCommand(usergroupDeleteCmd)
}
