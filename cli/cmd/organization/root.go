package organization

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "organization",
	Short: "Manage Netmaker organizations",
	Long:  `Manage Netmaker organizations`,
}

func GetRoot() *cobra.Command {
	return rootCmd
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
