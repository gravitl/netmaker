package user

import (
	"os"
	"strconv"
	"strings"

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
			table.SetHeader([]string{"Name", "Platform Role", "Groups"})
			for _, d := range *data {
				g := []string{}
				for gID := range d.UserGroups {
					g = append(g, gID.String())
				}
				table.Append([]string{d.UserName, d.PlatformRoleID.String(), strconv.FormatBool(d.IsAdmin), strings.Join(g, ",")})
			}
			table.Render()
		}
	},
}

func init() {
	rootCmd.AddCommand(userListCmd)
}
