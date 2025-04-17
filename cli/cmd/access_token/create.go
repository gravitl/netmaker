package access_token

import (
	"time"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/spf13/cobra"
)

var accessTokenCreateCmd = &cobra.Command{
	Use:   "create [token-name]",
	Short: "Create an access token",
	Long:  `Create an access token for a user`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		userName, _ := cmd.Flags().GetString("user")
		expiresAt, _ := cmd.Flags().GetString("expires")

		accessToken := &models.UserAccessToken{}
		accessToken.Name = args[0]
		accessToken.UserName = userName

		expTime := time.Now().Add(time.Hour * 24 * 365) // default to 1 year
		if expiresAt != "" {
			var err error
			expTime, err = time.Parse(time.RFC3339, expiresAt)
			if err != nil {
				cmd.PrintErrf("Invalid expiration time format. Please use RFC3339 format (e.g. 2024-01-01T00:00:00Z). Using default 1 year.\n")
			}
		}
		accessToken.ExpiresAt = expTime

		functions.PrettyPrint(functions.CreateAccessToken(accessToken))
	},
}

func init() {
	accessTokenCreateCmd.Flags().String("user", "", "Username to create token for")
	accessTokenCreateCmd.Flags().String("expires", "", "Expiration time for the token in RFC3339 format (e.g. 2024-01-01T00:00:00Z). Defaults to 1 year from now.")
	accessTokenCreateCmd.MarkFlagRequired("user")
	rootCmd.AddCommand(accessTokenCreateCmd)
}
