package acl

import (
	"os"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/guumaster/tablewriter"
	"github.com/spf13/cobra"
)

var aclListCmd = &cobra.Command{
	Use:   "list [NETWORK NAME]",
	Args:  cobra.ExactArgs(1),
	Short: "List all ACLs associated with a network",
	Long:  `List all ACLs associated with a network`,
	Run: func(cmd *cobra.Command, args []string) {
		aclSource := (map[acls.AclID]acls.ACL)(*functions.GetACL(args[0]))
		nodes := functions.GetNodes(args[0])
		idNameMap := make(map[string]string)
		for _, node := range *nodes {
			idNameMap[node.ID] = node.Name
		}
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"From", "To", "Status"})
		for id, acl := range aclSource {
			for k, v := range (map[acls.AclID]byte)(acl) {
				row := []string{idNameMap[string(id)], idNameMap[string(k)]}
				switch v {
				case acls.NotAllowed:
					row = append(row, "Not Allowed")
				case acls.NotPresent:
					row = append(row, "Not Present")
				case acls.Allowed:
					row = append(row, "Allowed")
				}
				table.Append(row)
			}
		}
		table.Render()
	},
}

func init() {
	rootCmd.AddCommand(aclListCmd)
}
