package cmd

import (
	"fmt"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var getLogsCmd = &cobra.Command{
	Use:   "logs",
	Args:  cobra.NoArgs,
	Short: "Retrieve server logs",
	Long:  `Retrieve server logs`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(functions.GetLogs())
	},
}

func init() {
	rootCmd.AddCommand(getLogsCmd)
}
