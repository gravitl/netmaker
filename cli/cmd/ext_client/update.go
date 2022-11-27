package ext_client

import (
	"encoding/json"
	"log"
	"os"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/spf13/cobra"
)

var extClientUpdateCmd = &cobra.Command{
	Use:   "update [NETWORK NAME] [NODE ID] [/path/to/ext_client_definition.json]",
	Args:  cobra.ExactArgs(3),
	Short: "Update an External Client",
	Long:  `Update an External Client`,
	Run: func(cmd *cobra.Command, args []string) {
		extClient := &models.ExtClient{}
		content, err := os.ReadFile(args[2])
		if err != nil {
			log.Fatal("Error when opening file: ", err)
		}
		if err := json.Unmarshal(content, extClient); err != nil {
			log.Fatal(err)
		}
		functions.PrettyPrint(functions.UpdateExtClient(args[0], args[1], extClient))
	},
}

func init() {
	rootCmd.AddCommand(extClientUpdateCmd)
}
