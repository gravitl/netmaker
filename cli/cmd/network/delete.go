package network

import (
	"fmt"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var networkDeleteCmd = &cobra.Command{
	Use:   "delete [NAME]",
	Short: "Delete a Network",
	Long:  `Delete a Network`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(*functions.DeleteNetwork(args[0]))
	},
}

func init() {
	rootCmd.AddCommand(networkDeleteCmd)
}
