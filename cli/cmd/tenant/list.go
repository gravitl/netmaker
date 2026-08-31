package tenant

import (
	"os"

	"github.com/gravitl/netmaker/cli/cmd/commons"
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/guumaster/tablewriter"
	"github.com/spf13/cobra"
)

var orgID string

var tenantListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tenants in an organization",
	Long:  `List tenants in an organization (requires --org_id or context org_id; sent as X-Organization-ID)`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		tenants := functions.ListTenants(orgID)
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
	tenantListCmd.Flags().StringVar(&orgID, "org_id", "", "Organization ID (sent as X-Organization-ID; falls back to context org_id)")
	rootCmd.AddCommand(tenantListCmd)
}
