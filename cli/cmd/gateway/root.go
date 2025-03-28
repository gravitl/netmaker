package gateway

import (
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:     "gateway",
	Short:   "Manage Gateways.",
	Long:    `Manage Gateways.`,
	Aliases: []string{"gw"},
}

// GetRoot returns the root subcommand.
func GetRoot() *cobra.Command {
	return rootCmd
}
