package server

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var serverInfoCmd = &cobra.Command{
	Use:   "info",
	Args:  cobra.NoArgs,
	Short: "Retrieve server information",
	Long:  `Retrieve server information`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.GetServerInfo())
	},
}

func init() {
	rootCmd.AddCommand(serverInfoCmd)
}
