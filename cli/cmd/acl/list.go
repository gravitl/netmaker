package acl

import (
	"github.com/gravitl/netmaker/logic/nodeacls"
	"os"

	"github.com/gravitl/netmaker/cli/cmd/commons"
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/guumaster/tablewriter"
	"github.com/spf13/cobra"
)

var aclListCmd = &cobra.Command{
	Use:   "list [NETWORK NAME]",
	Args:  cobra.ExactArgs(1),
	Short: "List all ACLs associated with a network",
	Long:  `List all ACLs associated with a network`,
	Run: func(cmd *cobra.Command, args []string) {
		aclSource := (map[nodeacls.AclID]nodeacls.ACL)(*functions.GetACL(args[0]))
		switch commons.OutputFormat {
		case commons.JsonOutput:
			functions.PrettyPrint(aclSource)
		default:
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"From", "To", "Status"})
			for id, acl := range aclSource {
				for k, v := range (map[nodeacls.AclID]byte)(acl) {
					row := []string{string(id), string(k)}
					switch v {
					case nodeacls.NotAllowed:
						row = append(row, "Not Allowed")
					case nodeacls.NotPresent:
						row = append(row, "Not Present")
					case nodeacls.Allowed:
						row = append(row, "Allowed")
					}
					table.Append(row)
				}
			}
			table.Render()
		}
	},
}

func init() {
	rootCmd.AddCommand(aclListCmd)
}
