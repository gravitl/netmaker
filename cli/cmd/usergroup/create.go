package usergroup

import (
	"fmt"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var usergroupCreateCmd = &cobra.Command{
	Use:   "create [GROUP NAME]",
	Args:  cobra.ExactArgs(1),
	Short: "Create a usergroup",
	Long:  `Create a usergroup`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.CreateUsergroup(args[0])
		fmt.Println("Success")
	},
}

func init() {
	rootCmd.AddCommand(usergroupCreateCmd)
}
