package acl

import (
	"fmt"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/spf13/cobra"
)

var aclAllowCmd = &cobra.Command{
	Use:   "allow [NETWORK NAME] [NODE_1_ID] [NODE_2_ID]",
	Args:  cobra.ExactArgs(3),
	Short: "Allow access from one node to another",
	Long:  `Allow access from one node to another`,
	Run: func(cmd *cobra.Command, args []string) {
		fromNodeID := args[1]
		toNodeID := args[2]
		payload := acls.ACLContainer(map[acls.AclID]acls.ACL{
			acls.AclID(fromNodeID): map[acls.AclID]byte{
				acls.AclID(toNodeID): acls.Allowed,
			},
			acls.AclID(toNodeID): map[acls.AclID]byte{
				acls.AclID(fromNodeID): acls.Allowed,
			},
		})
		functions.UpdateACL(args[0], &payload)
		fmt.Println("Success")
	},
}

func init() {
	rootCmd.AddCommand(aclAllowCmd)
}
