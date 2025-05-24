package host

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var hostRefreshKeysCmd = &cobra.Command{
	Use:   "refresh_keys [DEVICE ID/HOST ID]",
	Args:  cobra.MaximumNArgs(1),
	Short: "Refresh wireguard keys on device",
	Long: `Refresh wireguard keys on specified or all devices
	If DEVICE ID/HOST ID is not specified, all devices will be updated`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.RefreshKeys(args[0]))
	},
}

func init() {
	rootCmd.AddCommand(hostRefreshKeysCmd)
}
