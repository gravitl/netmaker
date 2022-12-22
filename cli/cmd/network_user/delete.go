package network_user

import (
	"fmt"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var networkuserDeleteCmd = &cobra.Command{
	Use:   "delete [NETWORK NAME] [NETWORK USER NAME]",
	Args:  cobra.ExactArgs(2),
	Short: "Delete a network user",
	Long:  `Delete a network user`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.DeleteNetworkUser(args[0], args[1])
		fmt.Println("Success")
	},
}

func init() {
	rootCmd.AddCommand(networkuserDeleteCmd)
}
