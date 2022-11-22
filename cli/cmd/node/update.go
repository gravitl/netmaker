package node

import (
	"encoding/json"
	"log"
	"os"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/spf13/cobra"
)

var nodeUpdateCmd = &cobra.Command{
	Use:   "update [NETWORK NAME] [NODE ID] [/path/to/node_definition.json]",
	Args:  cobra.ExactArgs(3),
	Short: "Update a Node",
	Long:  `Update a Node`,
	Run: func(cmd *cobra.Command, args []string) {
		content, err := os.ReadFile(args[2])
		if err != nil {
			log.Fatal("Error when opening file: ", err)
		}
		node := &models.Node{}
		if err := json.Unmarshal(content, node); err != nil {
			log.Fatal(err)
		}
		functions.PrettyPrint(functions.UpdateNode(args[0], args[1], node))
	},
}

func init() {
	rootCmd.AddCommand(nodeUpdateCmd)
}
