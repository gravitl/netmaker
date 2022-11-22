package dns

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var dnsDeleteCmd = &cobra.Command{
	Use:   "delete [NETWORK NAME] [DOMAIN NAME]",
	Args:  cobra.ExactArgs(2),
	Short: "Delete a DNS entry",
	Long:  `Delete a DNS entry`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.DeleteDNS(args[0], args[1]))
	},
}

func init() {
	rootCmd.AddCommand(dnsDeleteCmd)
}
