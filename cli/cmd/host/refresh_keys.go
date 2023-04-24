package host

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var hostRefreshKeysCmd = &cobra.Command{
	Use:   "refresh_keys [HOST ID] ",
	Args:  cobra.MaximumNArgs(1),
	Short: "Refresh wireguard keys on host",
	Long: `Refresh wireguard keys on specified or all hosts
	If HOSTID is not specified, all hosts will be updated`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.RefreshKeys(args[0]))
	},
}

func init() {
	rootCmd.AddCommand(hostRefreshKeysCmd)
}
