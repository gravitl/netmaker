package context

import (
	"fmt"
	"log"

	"github.com/gravitl/netmaker/cli/config"
	"github.com/spf13/cobra"
)

const (
	FlagEndpoint  = "endpoint"
	FlagUsername  = "username"
	FlagPassword  = "password"
	FlagMasterKey = "master_key"
)

var (
	endpoint  string
	username  string
	password  string
	masterKey string
)

// contextSetCmd creates/updates a context
var contextSetCmd = &cobra.Command{
	Use:   fmt.Sprintf("set [NAME] [--%s=https://api.netmaker.io] [--%s=admin] [--%s=pass] [--%s=secret]", FlagEndpoint, FlagUsername, FlagPassword, FlagMasterKey),
	Args:  cobra.ExactArgs(1),
	Short: "Create a context or update an existing one",
	Long:  `Create a context or update an existing one`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := config.Context{
			Endpoint:  endpoint,
			Username:  username,
			Password:  password,
			MasterKey: masterKey,
		}
		if ctx.Username == "" && ctx.MasterKey == "" {
			cmd.Usage()
			log.Fatal("Either username/password or master key is required")
		}
		config.SetContext(args[0], ctx)
	},
}

func init() {
	contextSetCmd.Flags().StringVar(&endpoint, FlagEndpoint, "", "Endpoint of the API Server")
	contextSetCmd.Flags().StringVar(&username, FlagUsername, "", "Username")
	contextSetCmd.Flags().StringVar(&password, FlagPassword, "", "Password")
	contextSetCmd.MarkFlagsRequiredTogether(FlagUsername, FlagPassword)
	contextSetCmd.Flags().StringVar(&masterKey, FlagMasterKey, "", "Master Key")

	rootCmd.AddCommand(contextSetCmd)
}
