package network

import (
	"encoding/json"
	"log"
	"os"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/spf13/cobra"
)

// networkCreateCmd represents the networkCreate command
var networkCreateCmd = &cobra.Command{
	Use:   "create [/path/to/network_definition.json]",
	Short: "Create a Network",
	Long:  `Create a Network`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		content, err := os.ReadFile(args[0])
		if err != nil {
			log.Fatal("Error when opening file: ", err)
		}
		network := &models.Network{}
		if err := json.Unmarshal(content, network); err != nil {
			log.Fatal(err)
		}
		functions.PrettyPrint(functions.CreateNetwork(network))
	},
}

func init() {
	rootCmd.AddCommand(networkCreateCmd)
}
