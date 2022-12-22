package acl

import (
	"fmt"
	"log"
	"strings"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/spf13/cobra"
)

var aclAllowCmd = &cobra.Command{
	Use:   "allow [NETWORK NAME] [FROM_NODE_NAME] [TO_NODE_NAME]",
	Args:  cobra.ExactArgs(3),
	Short: "Allow access from one node to another",
	Long:  `Allow access from one node to another`,
	Run: func(cmd *cobra.Command, args []string) {
		nameIDMap := make(map[string]string)
		for _, node := range *functions.GetNodes(args[0]) {
			nameIDMap[strings.ToLower(node.Name)] = node.ID
		}
		fromNodeID, ok := nameIDMap[strings.ToLower(args[1])]
		if !ok {
			log.Fatalf("Node %s doesn't exist", args[1])
		}
		toNodeID, ok := nameIDMap[strings.ToLower(args[2])]
		if !ok {
			log.Fatalf("Node %s doesn't exist", args[2])
		}
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
