package network_user

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var networkName string

var networkuserListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.NoArgs,
	Short: "List network users",
	Long:  `List network users`,
	Run: func(cmd *cobra.Command, args []string) {
		if networkName != "" {
			functions.PrettyPrint(functions.GetNetworkUsers(networkName))
		} else {
			functions.PrettyPrint(functions.GetAllNetworkUsers())
		}
	},
}

func init() {
	networkuserListCmd.Flags().StringVar(&networkName, "network", "", "Name of the network")
	rootCmd.AddCommand(networkuserListCmd)
}
