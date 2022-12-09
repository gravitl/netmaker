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
		user := &models.User{UserName: args[0], IsAdmin: admin}
		if networks != "" {
			user.Networks = strings.Split(networks, ",")
		}
		if groups != "" {
			user.Groups = strings.Split(groups, ",")
		} else {
			user.Groups = []string{"*"}
		}
		functions.PrettyPrint(functions.UpdateUser(user))
	},
}

func init() {
	userUpdateCmd.Flags().BoolVar(&admin, "admin", false, "Make the user an admin ?")
	userUpdateCmd.Flags().StringVar(&networks, "networks", "", "List of networks the user will access to (comma separated)")
	userUpdateCmd.Flags().StringVar(&groups, "groups", "", "List of user groups the user will be part of (comma separated)")
	rootCmd.AddCommand(userUpdateCmd)
}
