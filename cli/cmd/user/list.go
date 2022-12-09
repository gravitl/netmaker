package user

import (
	"os"
	"strconv"
	"strings"

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
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Name", "Admin", "Networks", "Groups"})
		for _, d := range *functions.ListUsers() {
			table.Append([]string{d.UserName, strconv.FormatBool(d.IsAdmin), strings.Join(d.Networks, ", "), strings.Join(d.Groups, ", ")})
		}
		table.Render()
	},
}

func init() {
	rootCmd.AddCommand(userListCmd)
}
