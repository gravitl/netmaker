package acl

import (
	"fmt"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/spf13/cobra"
)

var aclDenyCmd = &cobra.Command{
	Use:   "deny [NETWORK NAME] [FROM_NODE_NAME] [TO_NODE_NAME]",
	Args:  cobra.ExactArgs(3),
	Short: "Deny access from one node to another",
	Long:  `Deny access from one node to another`,
	Run: func(cmd *cobra.Command, args []string) {
		fromNodeID := args[1]
		toNodeID := args[2]
		payload := acls.ACLContainer(map[acls.AclID]acls.ACL{
			acls.AclID(fromNodeID): map[acls.AclID]byte{
				acls.AclID(toNodeID): acls.NotAllowed,
			},
			acls.AclID(toNodeID): map[acls.AclID]byte{
				acls.AclID(fromNodeID): acls.NotAllowed,
			},
		})
		functions.UpdateACL(args[0], &payload)
		fmt.Println("Success")
	},
}

func init() {
	rootCmd.AddCommand(aclDenyCmd)
}
