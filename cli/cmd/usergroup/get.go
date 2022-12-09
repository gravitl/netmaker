package usergroup

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var usergroupGetCmd = &cobra.Command{
	Use:   "get",
	Args:  cobra.NoArgs,
	Short: "Fetch all usergroups",
	Long:  `Fetch all usergroups`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.GetUsergroups())
	},
}

func init() {
	rootCmd.AddCommand(usergroupGetCmd)
}
