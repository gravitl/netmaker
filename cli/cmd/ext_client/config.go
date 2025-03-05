package ext_client

import (
	"fmt"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var extClientConfigCmd = &cobra.Command{
	Use:   "config [NETWORK NAME] [EXTERNAL CLIENT ID]",
	Args:  cobra.ExactArgs(2),
	Short: "Get an External Client Configuration",
	Long:  `Get an External Client Configuration`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(functions.GetExtClientConfig(args[0], args[1]))
	},
}

var extClientHAConfigCmd = &cobra.Command{
	Use:   "auto_config [NETWORK NAME]",
	Args:  cobra.ExactArgs(1),
	Short: "Get an External Client Configuration",
	Long:  `Get an External Client Configuration`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(functions.GetExtClientHAConfig(args[0]))
	},
}

func init() {
	rootCmd.AddCommand(extClientConfigCmd)
	rootCmd.AddCommand(extClientHAConfigCmd)
}
