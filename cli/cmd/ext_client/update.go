package ext_client

import (
	"encoding/json"
	"log"
	"os"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/spf13/cobra"
)

var (
	extClientUpdateFile string
)

var extClientUpdateCmd = &cobra.Command{
	Use:   "update [NETWORK NAME] [EXTERNAL CLIENT ID]",
	Args:  cobra.ExactArgs(2),
	Short: "Update an External Client",
	Long:  `Update an External Client`,
	Run: func(cmd *cobra.Command, args []string) {
		var (
			network   = args[0]
			clientID  = args[1]
			extClient = &models.CustomExtClient{}
		)
		if extClientUpdateFile != "" {
			content, err := os.ReadFile(extClientUpdateFile)
			if err != nil {
				log.Fatal("Error when opening file: ", err)
			}
			if err := json.Unmarshal(content, extClient); err != nil {
				log.Fatal(err)
			}
		} else {
			extClient.ClientID = extClientID
			extClient.PublicKey = publicKey
			extClient.DNS = dns
		}
		functions.PrettyPrint(functions.UpdateExtClient(network, clientID, extClient))
	},
}

func init() {
	extClientUpdateCmd.Flags().StringVar(&extClientID, "id", "", "updated ID of the external client")
	extClientUpdateCmd.Flags().StringVar(&extClientUpdateFile, "file", "", "Filepath of updated external client definition in JSON")
	extClientUpdateCmd.Flags().StringVar(&publicKey, "public_key", "", "updated public key of the external client")
	extClientUpdateCmd.Flags().StringVar(&dns, "dns", "", "updated DNS of the external client")
	extClientUpdateCmd.Flags().StringSliceVar(&allowedips, "allowedips", []string{}, "updated extra allowed IPs of the external client")
	rootCmd.AddCommand(extClientUpdateCmd)
}
