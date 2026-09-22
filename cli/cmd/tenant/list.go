package tenant

import (
	"os"

	"github.com/gravitl/netmaker/cli/cmd/commons"
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/guumaster/tablewriter"
	"github.com/spf13/cobra"
)

var tenantListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tenants in an organization",
	Long:  `List tenants in the organization configured via context org_id (X-Organization-ID)`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		tenants := functions.ListTenants()
		switch commons.OutputFormat {
		case commons.JsonOutput:
			functions.PrettyPrint(tenants)
		default:
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Name", "Slug", "Organization ID"})
			for _, t := range tenants {
				table.Append([]string{t.ID, t.Name, t.Slug, t.OrganizationID})
			}
			table.Render()
		}
	},
}

func init() {
	rootCmd.AddCommand(tenantListCmd)
}
