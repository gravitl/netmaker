package server

import (
	"fmt"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var serverHasAdminCmd = &cobra.Command{
	Use:   "has_admin",
	Args:  cobra.NoArgs,
	Short: "Check if server has an admin",
	Long:  `Check if server has an admin`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(*functions.HasAdmin())
	},
}

func init() {
	rootCmd.AddCommand(serverHasAdminCmd)
}
