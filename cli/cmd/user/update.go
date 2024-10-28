package user

import (
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

	userUpdateCmd.Flags().StringVar(&password, "password", "", "Password of the user")
	userUpdateCmd.Flags().StringVarP(&platformID, "platform-role", "r", "",
		"Platform Role of the user; run `nmctl roles list` to see available user roles")
	userUpdateCmd.PersistentFlags().StringToStringVarP(&networkRoles, "network-roles", "n", nil,
		"Mapping of networkID and list of roles user will be part of (comma separated)")
	userUpdateCmd.Flags().BoolVar(&admin, "admin", false, "Make the user an admin ? (deprecated v0.25.0 onwards)")
	userUpdateCmd.Flags().StringArrayVarP(&groups, "groups", "g", nil, "List of user groups the user will be part of (comma separated)")
	rootCmd.AddCommand(userUpdateCmd)
}
