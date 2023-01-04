package context

import (
	"github.com/gravitl/netmaker/cli/config"
	"github.com/spf13/cobra"
)

var contextListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.NoArgs,
	Short: "List all contexts",
	Long:  `List all contexts`,
	Run: func(cmd *cobra.Command, args []string) {
		config.ListAll()
	},
}

func init() {
	rootCmd.AddCommand(contextListCmd)
}
