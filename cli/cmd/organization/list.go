package organization

import (
	"os"

	"github.com/gravitl/netmaker/cli/cmd/commons"
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/guumaster/tablewriter"
	"github.com/spf13/cobra"
)

var organizationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List organizations",
	Long:  `List organizations visible to the current user`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		orgs := functions.ListOrganizations()
		switch commons.OutputFormat {
		case commons.JsonOutput:
			functions.PrettyPrint(orgs)
		default:
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Name", "Slug"})
			for _, o := range orgs {
				table.Append([]string{o.ID, o.Name, o.Slug})
			}
			table.Render()
		}
	},
}

func init() {
	rootCmd.AddCommand(organizationListCmd)
}
