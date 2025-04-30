package acl

import (
	"fmt"
	"github.com/gravitl/netmaker/logic/nodeacls"
	"log"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var aclDenyCmd = &cobra.Command{
	Use:   "deny [NETWORK NAME] [NODE_1_ID] [NODE_2_ID]",
	Args:  cobra.ExactArgs(3),
	Short: "Deny access from one node to another",
	Long:  `Deny access from one node to another`,
	Run: func(cmd *cobra.Command, args []string) {
		network := args[0]
		fromNodeID := args[1]
		toNodeID := args[2]

		if fromNodeID == toNodeID {
			log.Fatal("Cannot deny access to self")
		}

		// get current acls
		res := functions.GetACL(network)
		if res == nil {
			log.Fatalf("Could not load network ACLs")
		}

		payload := *res

		if _, ok := payload[nodeacls.AclID(fromNodeID)]; !ok {
			log.Fatalf("Node [%s] does not exist", fromNodeID)
		}
		if _, ok := payload[nodeacls.AclID(toNodeID)]; !ok {
			log.Fatalf("Node [%s] does not exist", toNodeID)
		}

		// update acls
		payload[nodeacls.AclID(fromNodeID)][nodeacls.AclID(toNodeID)] = nodeacls.NotAllowed
		payload[nodeacls.AclID(toNodeID)][nodeacls.AclID(fromNodeID)] = nodeacls.NotAllowed

		functions.UpdateACL(network, &payload)
		fmt.Println("Success")
	},
}

func init() {
	rootCmd.AddCommand(aclDenyCmd)
}
