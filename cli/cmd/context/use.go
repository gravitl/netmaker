package context

import (
	"github.com/gravitl/netmaker/cli/config"
	"github.com/spf13/cobra"
)

var contextUseCmd = &cobra.Command{
	Use:   "use [NAME]",
	Args:  cobra.ExactArgs(1),
	Short: "Set the current context",
	Long:  `Set the current context`,
	Run: func(cmd *cobra.Command, args []string) {
		config.SetCurrentContext(args[0])
	},
}

func init() {
	rootCmd.AddCommand(contextUseCmd)
}
