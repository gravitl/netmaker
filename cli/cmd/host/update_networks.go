package host

import (
	"strings"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var hostUpdateNetworksCmd = &cobra.Command{
	Use:   "update_network HostID Networks(comma separated list)",
	Args:  cobra.ExactArgs(2),
	Short: "Update a host's networks",
	Long:  `Update a host's networks`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.UpdateHostNetworks(args[0], strings.Split(args[1], ",")))
	},
}

func init() {
	rootCmd.AddCommand(hostUpdateNetworksCmd)
}
