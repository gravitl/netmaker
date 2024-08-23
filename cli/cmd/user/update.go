package user

import (
	"strings"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/spf13/cobra"
)

var userUpdateCmd = &cobra.Command{
	Use:   "update [USER NAME]",
	Args:  cobra.ExactArgs(1),
	Short: "Update a user",
	Long:  `Update a user`,
	Run: func(cmd *cobra.Command, args []string) {
		user := &models.User{UserName: args[0]}
		if platformID != "" {
			user.PlatformRoleID = models.UserRoleID(platformID)
		}
		if len(networkRoles) > 0 {
			netRolesMap := make(map[models.NetworkID]map[models.UserRoleID]struct{})
			for netID, netRoles := range networkRoles {
				roleMap := make(map[models.UserRoleID]struct{})
				for _, roleID := range strings.Split(netRoles, ",") {
					roleMap[models.UserRoleID(roleID)] = struct{}{}
				}
				netRolesMap[models.NetworkID(netID)] = roleMap
			}
			user.NetworkRoles = netRolesMap
		}
		if len(groups) > 0 {
			grMap := make(map[models.UserGroupID]struct{})
			for _, groupID := range groups {
				grMap[models.UserGroupID(groupID)] = struct{}{}
			}
			user.UserGroups = grMap
		}
		functions.PrettyPrint(functions.UpdateUser(user))
	},
}

func init() {
	userUpdateCmd.Flags().BoolVar(&admin, "admin", false, "Make the user an admin ?")
	userUpdateCmd.Flags().StringVar(&networks, "networks", "", "List of networks the user will access to (comma separated)")
	userUpdateCmd.Flags().StringVarP(&platformID, "platform-id", "r", "",
		"Platform Role of the user; run `nmctl roles list` to see available user roles")
	userUpdateCmd.Flags().StringArrayVarP(&groups, "groups", "g", nil, "List of user groups the user will be part of (comma separated)")
	rootCmd.AddCommand(userUpdateCmd)
}
