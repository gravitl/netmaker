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
			SSO:       sso,
			TenantId:  tenantId,
			Saas:      saas,
		}
		if !ctx.Saas {
			if ctx.Username == "" && ctx.MasterKey == "" && !ctx.SSO {
				cmd.Usage()
				log.Fatal("Either username/password or master key is required")
			}
			if ctx.Endpoint == "" {
				cmd.Usage()
				log.Fatal("Endpoint is required when for self-hosted tenants")
			}
		} else {
			if ctx.TenantId == "" {
				cmd.Usage()
				log.Fatal("Tenant ID is required for SaaS tenants")
			}
			ctx.Endpoint = fmt.Sprintf(functions.TenantUrlTemplate, tenantId)
		}
		config.SetContext(args[0], ctx)
	},
}

func init() {
	contextSetCmd.Flags().StringVar(&endpoint, "endpoint", "", "Endpoint of the API Server")
	contextSetCmd.Flags().StringVar(&username, "username", "", "Username")
	contextSetCmd.Flags().StringVar(&password, "password", "", "Password")
	contextSetCmd.MarkFlagsRequiredTogether("username", "password")
	contextSetCmd.Flags().BoolVar(&sso, "sso", false, "Login via Single Sign On (SSO)?")
	contextSetCmd.Flags().StringVar(&masterKey, "master_key", "", "Master Key")
	contextSetCmd.Flags().StringVar(&tenantId, "tenant_id", "", "Tenant ID")
	contextSetCmd.Flags().BoolVar(&saas, "saas", false, "Is this context for a SaaS tenant?")
	rootCmd.AddCommand(contextSetCmd)
}
