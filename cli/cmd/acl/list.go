package acl

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var aclListCmd = &cobra.Command{
	Use:   "list [NETWORK NAME]",
	Args:  cobra.ExactArgs(1),
	Short: "List all ACLs associated with a network",
	Long:  `List all ACLs associated with a network`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.GetACL(args[0]))
	},
}

func init() {
	rootCmd.AddCommand(aclListCmd)
}
