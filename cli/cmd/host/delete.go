package host

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var force bool

var hostDeleteCmd = &cobra.Command{
	Use:   "delete DeviceID/HostID",
	Args:  cobra.ExactArgs(1),
	Short: "Delete a device",
	Long:  `Delete a device`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.DeleteHost(args[0], force))
	},
}

func init() {
	rootCmd.AddCommand(hostDeleteCmd)
	hostDeleteCmd.PersistentFlags().BoolVarP(&force, "force", "f", false, "delete even if part of network(s)")
}
