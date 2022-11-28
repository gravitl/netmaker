package context

import (
	"github.com/gravitl/netmaker/cli/config"
	"github.com/spf13/cobra"
)

var contextDeleteCmd = &cobra.Command{
	Use:   "delete [NAME]",
	Args:  cobra.ExactArgs(1),
	Short: "Delete a context",
	Long:  `Delete a context`,
	Run: func(cmd *cobra.Command, args []string) {
		config.DeleteContext(args[0])
	},
}

func init() {
	rootCmd.AddCommand(contextDeleteCmd)
}
