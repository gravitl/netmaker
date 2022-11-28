package user

import (
	"strings"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/spf13/cobra"
)

var userCreateCmd = &cobra.Command{
	Use:   "create",
	Args:  cobra.NoArgs,
	Short: "Create a new user",
	Long:  `Create a new user`,
	Run: func(cmd *cobra.Command, args []string) {
		user := &models.User{UserName: username, Password: password, IsAdmin: admin}
		if networks != "" {
			user.Networks = strings.Split(networks, ",")
		}
		if groups != "" {
			user.Groups = strings.Split(groups, ",")
		}
		functions.PrettyPrint(functions.CreateUser(user))
	},
}

func init() {
	userCreateCmd.Flags().StringVar(&username, "name", "", "Name of the user")
	userCreateCmd.Flags().StringVar(&password, "password", "", "Password of the user")
	userCreateCmd.MarkFlagRequired("name")
	userCreateCmd.MarkFlagRequired("password")
	userCreateCmd.Flags().BoolVar(&admin, "admin", false, "Make the user an admin ?")
	userCreateCmd.Flags().StringVar(&networks, "networks", "", "List of networks the user will access to (comma separated)")
	userCreateCmd.Flags().StringVar(&groups, "groups", "", "List of user groups the user will be part of (comma separated)")
	rootCmd.AddCommand(userCreateCmd)
}
