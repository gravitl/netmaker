package user

import (
	"fmt"
	"os"
	"strconv"

	"github.com/gravitl/netmaker/cli/cmd/commons"
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
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
var userroleListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.NoArgs,
	Short: "List all user roles",
	Long:  `List all user roles`,
	Run: func(cmd *cobra.Command, args []string) {
		data := functions.ListUserRoles()
		userRoles := data.Response.([]models.UserRolePermissionTemplate)
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
			for _, d := range userRoles {
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
		fmt.Println("CLI doesn't support creation of roles currently")
	},
}

func init() {
	rootCmd.AddCommand(userRoleCmd)
	userroleListCmd.Flags().BoolVar(&platformRoles, "platform-roles", true,
		"set to false to list network roles. By default it will only list platform roles")
	userRoleCmd.AddCommand(userroleListCmd)
	userRoleCmd.AddCommand(userCreateCmd)
}
