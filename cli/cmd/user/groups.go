package user

import (
	"fmt"
	"os"
	"strings"

	"github.com/gravitl/netmaker/cli/cmd/commons"
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/guumaster/tablewriter"
	"github.com/spf13/cobra"
)

var userGroupCmd = &cobra.Command{
	Use:   "group",
	Args:  cobra.NoArgs,
	Short: "Manage User Groups",
	Long:  `Manage User Groups`,
}

var userGroupListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.NoArgs,
	Short: "List all user groups",
	Long:  `List all user groups`,
	Run: func(cmd *cobra.Command, args []string) {
		data := functions.ListUserGrps()
		switch commons.OutputFormat {
		case commons.JsonOutput:
			functions.PrettyPrint(data)
		default:
			table := tablewriter.NewWriter(os.Stdout)
			h := []string{"ID", "MetaData", "Network Roles"}
			table.SetHeader(h)
			for _, d := range data {

				roleInfoStr := ""
				for netID, netRoleMap := range d.NetworkRoles {
					roleList := []string{}
					for roleID := range netRoleMap {
						roleList = append(roleList, roleID.String())
					}
					roleInfoStr += fmt.Sprintf("[%s]: %s", netID, strings.Join(roleList, ","))
				}
				e := []string{d.ID.String(), d.MetaData, roleInfoStr}
				table.Append(e)
			}
			table.Render()
		}
	},
}

var userGroupCreateCmd = &cobra.Command{
	Use:   "create",
	Args:  cobra.NoArgs,
	Short: "create user group",
	Long:  `create user group`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("CLI doesn't support creation of groups currently. Visit the dashboard to create one or refer to our api documentation https://docs.v2.netmaker.io/reference")
	},
}

var userGroupDeleteCmd = &cobra.Command{
	Use:   "delete [groupID]",
	Args:  cobra.ExactArgs(1),
	Short: "delete user group",
	Long:  `delete user group`,
	Run: func(cmd *cobra.Command, args []string) {
		resp := functions.DeleteUserGrp(args[0])
		if resp != nil {
			fmt.Println(resp.Message)
		}
	},
}

var userGroupGetCmd = &cobra.Command{
	Use:   "get [groupID]",
	Args:  cobra.ExactArgs(1),
	Short: "get user group",
	Long:  `get user group`,
	Run: func(cmd *cobra.Command, args []string) {
		data := functions.GetUserGrp(args[0])
		switch commons.OutputFormat {
		case commons.JsonOutput:
			functions.PrettyPrint(data)
		default:
			table := tablewriter.NewWriter(os.Stdout)
			h := []string{"ID", "MetaData", "Network Roles"}
			table.SetHeader(h)
			roleInfoStr := ""
			for netID, netRoleMap := range data.NetworkRoles {
				roleList := []string{}
				for roleID := range netRoleMap {
					roleList = append(roleList, roleID.String())
				}
				roleInfoStr += fmt.Sprintf("[%s]: %s", netID, strings.Join(roleList, ","))
			}
			e := []string{data.ID.String(), data.MetaData, roleInfoStr}
			table.Append(e)
			table.Render()
		}
	},
}

func init() {
	rootCmd.AddCommand(userGroupCmd)
	// list roles cmd
	userGroupCmd.AddCommand(userGroupListCmd)

	// create roles cmd
	userGroupCmd.AddCommand(userGroupCreateCmd)

	// delete role cmd
	userGroupCmd.AddCommand(userGroupDeleteCmd)

	// Get Role
	userGroupCmd.AddCommand(userGroupGetCmd)
}
