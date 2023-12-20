package acl

import (
	"fmt"
	"log"

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
		network := args[0]
		fromNodeID := args[1]
		toNodeID := args[2]

		if fromNodeID == toNodeID {
			log.Fatal("Cannot allow access from a node to itself")
		}

		// get current acls
		res := functions.GetACL(network)
		if res == nil {
			log.Fatalf("Could not load network ACLs")
		}

		payload := *res

		if _, ok := payload[acls.AclID(fromNodeID)]; !ok {
			log.Fatalf("Node %s does not exist", fromNodeID)
		}
		if _, ok := payload[acls.AclID(toNodeID)]; !ok {
			log.Fatalf("Node %s does not exist", toNodeID)
		}

		// update acls
		payload[acls.AclID(fromNodeID)][acls.AclID(toNodeID)] = acls.Allowed
		payload[acls.AclID(toNodeID)][acls.AclID(fromNodeID)] = acls.Allowed

		functions.UpdateACL(network, &payload)
		fmt.Println("Success")
	},
}

func init() {
	rootCmd.AddCommand(aclAllowCmd)
}
