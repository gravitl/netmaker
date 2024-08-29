package user

import (
	"fmt"
	"os"
	"strconv"

	"github.com/gravitl/netmaker/cli/cmd/commons"
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/guumaster/tablewriter"
	"github.com/spf13/cobra"
)

var userRoleCmd = &cobra.Command{
	Use:   "role",
	Args:  cobra.NoArgs,
	Short: "Manage User Roles",
	Long:  `Manage User Roles`,
}

// List Roles
var (
	platformRoles bool
)
var userRoleListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.NoArgs,
	Short: "List all user roles",
	Long:  `List all user roles`,
	Run: func(cmd *cobra.Command, args []string) {
		data := functions.ListUserRoles()
		switch commons.OutputFormat {
		case commons.JsonOutput:
			functions.PrettyPrint(data)
		default:
			table := tablewriter.NewWriter(os.Stdout)
			h := []string{"ID", "Default", "Dashboard Access", "Full Access"}

			if !platformRoles {
				h = append(h, "Network")
			}
			table.SetHeader(h)
			for _, d := range data {
				e := []string{d.ID.String(), strconv.FormatBool(d.Default), strconv.FormatBool(d.DenyDashboardAccess), strconv.FormatBool(d.FullAccess)}
				if !platformRoles {
					e = append(e, d.NetworkID.String())
				}
				table.Append(e)
			}
			table.Render()
		}
	},
}

var userRoleCreateCmd = &cobra.Command{
	Use:   "create",
	Args:  cobra.NoArgs,
	Short: "create user role",
	Long:  `create user role`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("CLI doesn't support creation of roles currently. Visit the dashboard to create one or refer to our api documentation https://docs.v2.netmaker.io/reference")
	},
}

var userRoleDeleteCmd = &cobra.Command{
	Use:   "delete [roleID]",
	Args:  cobra.ExactArgs(1),
	Short: "delete user role",
	Long:  `delete user role`,
	Run: func(cmd *cobra.Command, args []string) {
		resp := functions.DeleteUserRole(args[0])
		if resp != nil {
			fmt.Println(resp.Message)
		}
	},
}

var userRoleGetCmd = &cobra.Command{
	Use:   "get [roleID]",
	Args:  cobra.ExactArgs(1),
	Short: "get user role",
	Long:  `get user role`,
	Run: func(cmd *cobra.Command, args []string) {
		d := functions.GetUserRole(args[0])
		switch commons.OutputFormat {
		case commons.JsonOutput:
			functions.PrettyPrint(d)
		default:
			table := tablewriter.NewWriter(os.Stdout)
			h := []string{"ID", "Default Role", "Dashboard Access", "Full Access"}

			if d.NetworkID != "" {
				h = append(h, "Network")
			}
			table.SetHeader(h)
			e := []string{d.ID.String(), strconv.FormatBool(d.Default), strconv.FormatBool(!d.DenyDashboardAccess), strconv.FormatBool(d.FullAccess)}
			if !platformRoles {
				e = append(e, d.NetworkID.String())
			}
			table.Append(e)
			table.Render()
		}
	},
}

func init() {
	rootCmd.AddCommand(userRoleCmd)
	// list roles cmd
	userRoleListCmd.Flags().BoolVar(&platformRoles, "platform-roles", true,
		"set to false to list network roles. By default it will only list platform roles")
	userRoleCmd.AddCommand(userRoleListCmd)

	// create roles cmd
	userRoleCmd.AddCommand(userRoleCreateCmd)

	// delete role cmd
	userRoleCmd.AddCommand(userRoleDeleteCmd)

	// Get Role
	userRoleCmd.AddCommand(userRoleGetCmd)
}
