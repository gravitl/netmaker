package user

import (
	"os"
	"strconv"

	"github.com/gravitl/netmaker/cli/cmd/commons"
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/guumaster/tablewriter"
	"github.com/spf13/cobra"
)

var userListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.NoArgs,
	Short: "List all users",
	Long:  `List all users`,
	Run: func(cmd *cobra.Command, args []string) {
		data := functions.ListUsers()
		switch commons.OutputFormat {
		case commons.JsonOutput:
			functions.PrettyPrint(data)
		default:
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"Name", "SuperAdmin", "Admin"})
			for _, d := range *data {
				table.Append([]string{d.UserName, strconv.FormatBool(d.IsSuperAdmin), strconv.FormatBool(d.IsAdmin)})
			}
			table.Render()
		}
	},
}

func init() {
	rootCmd.AddCommand(userListCmd)
}
