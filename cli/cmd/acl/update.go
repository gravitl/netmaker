package acl

import (
	"encoding/json"
	"log"
	"os"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/spf13/cobra"
)

var aclUpdatetCmd = &cobra.Command{
	Use:   "update [NETWORK NAME] [/path/to/updated_acl.json]",
	Args:  cobra.ExactArgs(2),
	Short: "Update an ACL associated with a network",
	Long:  `Update an ACL associated with a network`,
	Run: func(cmd *cobra.Command, args []string) {
		content, err := os.ReadFile(args[1])
		if err != nil {
			log.Fatal("Error when opening file: ", err)
		}
		acl := &acls.ACLContainer{}
		if err := json.Unmarshal(content, acl); err != nil {
			log.Fatal(err)
		}
		functions.PrettyPrint(functions.UpdateACL(args[0], acl))
	},
}

func init() {
	rootCmd.AddCommand(aclUpdatetCmd)
}
