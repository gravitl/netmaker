package network

import (
	"fmt"
	"os"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/spf13/cobra"
)

// networkCreateCmd represents the networkCreate command
var networkCreateCmd = &cobra.Command{
	Use:   "create [network_definition.json]",
	Short: "Create a Network",
	Long:  `Create a Network`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		network := &models.Network{}
		resp := functions.CreateNetwork(network)
		fmt.Fprintf(os.Stdout, "Response from `NetworksApi.CreateNetwork`: %v\n", resp)
	},
}

func init() {
	rootCmd.AddCommand(networkCreateCmd)
}
