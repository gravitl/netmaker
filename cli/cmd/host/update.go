package host

import (
	"encoding/json"
	"log"
	"os"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/spf13/cobra"
)

var hostUpdateCmd = &cobra.Command{
	Use:   "update HostID /path/to/host_definition.json",
	Args:  cobra.ExactArgs(2),
	Short: "Update a host",
	Long:  `Update a host`,
	Run: func(cmd *cobra.Command, args []string) {
		apiHost := &models.ApiHost{}
		content, err := os.ReadFile(args[1])
		if err != nil {
			log.Fatal("Error when opening file: ", err)
		}
		if err := json.Unmarshal(content, apiHost); err != nil {
			log.Fatal(err)
		}
		functions.PrettyPrint(functions.UpdateHost(args[0], apiHost))
	},
}

func init() {
	rootCmd.AddCommand(hostUpdateCmd)
}
