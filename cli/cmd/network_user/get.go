package network_user

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var data bool

var networkuserGetCmd = &cobra.Command{
	Use:   "get [NETWORK NAME] [NETWORK USER NAME]",
	Args:  cobra.ExactArgs(2),
	Short: "Fetch a network user",
	Long:  `Fetch a network user`,
	Run: func(cmd *cobra.Command, args []string) {
		if data {
			functions.PrettyPrint(functions.GetNetworkUserData(args[1]))
		} else {
			functions.PrettyPrint(functions.GetNetworkUser(args[0], args[1]))
		}
	},
}

func init() {
	networkuserGetCmd.Flags().BoolVar(&data, "data", false, "Fetch entire data of a network user")
	rootCmd.AddCommand(networkuserGetCmd)
}
