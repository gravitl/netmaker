package context

import (
	"fmt"
	"log"

	"github.com/gravitl/netmaker/cli/config"
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var (
	endpoint  string
	username  string
	password  string
	masterKey string
	sso       bool
	tenantId  string
	saas      bool
	authToken string
)

var contextSetCmd = &cobra.Command{
	Use:   "set [NAME]",
	Args:  cobra.ExactArgs(1),
	Short: "Create a context or update an existing one",
	Long:  `Create a context or update an existing one`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := config.Context{
			Endpoint:  endpoint,
			Username:  username,
			Password:  password,
			MasterKey: masterKey,
			AuthToken: authToken,
			SSO:       sso,
			TenantId:  tenantId,
			Saas:      saas,
		}
		if !ctx.Saas {
			if ctx.Username == "" && ctx.MasterKey == "" && !ctx.SSO && ctx.AuthToken == "" {
				log.Fatal("Either username/password or master key or auth token is required")
				cmd.Usage()
			}
			if ctx.Endpoint == "" {
				log.Fatal("Endpoint is required when for self-hosted tenants")
				cmd.Usage()
			}
		} else {
			if ctx.TenantId == "" {
				log.Fatal("Tenant ID is required for SaaS tenants")
				cmd.Usage()
			}
			ctx.Endpoint = fmt.Sprintf(functions.TenantUrlTemplate, tenantId)
			if ctx.Username == "" && ctx.Password == "" && ctx.AuthToken == "" && !ctx.SSO {
				log.Fatal("Username/password or authtoken is required for non-SSO SaaS contexts")
				cmd.Usage()
			}
		}
		config.SetContext(args[0], ctx)
	},
}

func init() {
	contextSetCmd.Flags().StringVar(&endpoint, "endpoint", "", "Endpoint of the API Server")
	contextSetCmd.Flags().StringVar(&username, "username", "", "Username")
	contextSetCmd.Flags().StringVar(&password, "password", "", "Password")
	contextSetCmd.Flags().StringVar(&authToken, "auth_token", "", "Auth Token")
	contextSetCmd.MarkFlagsRequiredTogether("username", "password")
	contextSetCmd.Flags().BoolVar(&sso, "sso", false, "Login via Single Sign On (SSO)?")
	contextSetCmd.Flags().StringVar(&masterKey, "master_key", "", "Master Key")
	contextSetCmd.Flags().StringVar(&tenantId, "tenant_id", "", "Tenant ID")
	contextSetCmd.Flags().BoolVar(&saas, "saas", false, "Is this context for a SaaS tenant?")
	rootCmd.AddCommand(contextSetCmd)
}
