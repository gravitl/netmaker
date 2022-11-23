package server

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var serverConfigCmd = &cobra.Command{
	Use:   "config",
	Args:  cobra.NoArgs,
	Short: "Retrieve server config",
	Long:  `Retrieve server config`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.GetServerConfig())
	},
}

func init() {
	rootCmd.AddCommand(serverConfigCmd)
}
