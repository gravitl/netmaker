package tenant

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tenant",
	Short: "Manage Netmaker tenants",
	Long:  `Manage Netmaker tenants`,
}

func GetRoot() *cobra.Command {
	return rootCmd
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
