package ext_client

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var networkName string

var extClientListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.NoArgs,
	Short: "List External Clients",
	Long:  `List External Clients`,
	Run: func(cmd *cobra.Command, args []string) {
		if networkName != "" {
			functions.PrettyPrint(functions.GetNetworkExtClients(networkName))
		} else {
			functions.PrettyPrint(functions.GetAllExtClients())
		}
	},
}

func init() {
	extClientListCmd.Flags().StringVar(&networkName, "network", "", "Network name")
	rootCmd.AddCommand(extClientListCmd)
}
