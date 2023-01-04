package network

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var networkRefreshKeysCmd = &cobra.Command{
	Use:   "refresh_keys [NETWORK NAME]",
	Short: "Refresh public and private key pairs of a network",
	Long:  `Refresh public and private key pairs of a network`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.RefreshKeys(args[0]))
	},
}

func init() {
	rootCmd.AddCommand(networkRefreshKeysCmd)
}
