package server

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var serverHealthCmd = &cobra.Command{
	Use:   "health",
	Args:  cobra.NoArgs,
	Short: "View server health",
	Long:  `View server health`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.GetServerHealth())
	},
}

func init() {
	rootCmd.AddCommand(serverHealthCmd)
}
