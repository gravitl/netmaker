package network

import (
	"encoding/json"
	"log"
	"os"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/spf13/cobra"
)

var networkUpdateCmd = &cobra.Command{
	Use:   "update [NAME] [/path/to/network_definition.json]",
	Short: "Update a Network",
	Long:  `Update a Network`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		content, err := os.ReadFile(args[1])
		if err != nil {
			log.Fatal("Error when opening file: ", err)
		}
		network := &models.Network{}
		if err := json.Unmarshal(content, network); err != nil {
			log.Fatal(err)
		}
		functions.PrettyPrint(functions.UpdateNetwork(args[0], network))
	},
}

func init() {
	rootCmd.AddCommand(networkUpdateCmd)
}
