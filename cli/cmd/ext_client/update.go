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
	extClientUpdateFile    string
	description            string
	privateKey             string
	publicKey              string
	address                string
	address6               string
	ingressGatewayID       string
	ingressGatewayEndpoint string
	ownerID                string
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
			extClient = &models.ExtClient{}
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
			extClient.ClientID = clientID
			extClient.Description = description
			extClient.PrivateKey = privateKey
			extClient.PublicKey = publicKey
			extClient.Network = network
			extClient.Address = address
			extClient.Address6 = address6
			extClient.IngressGatewayID = ingressGatewayID
			extClient.IngressGatewayEndpoint = ingressGatewayEndpoint
			extClient.OwnerID = ownerID
		}
		functions.PrettyPrint(functions.UpdateExtClient(network, clientID, extClient))
	},
}

func init() {
	extClientUpdateCmd.Flags().StringVar(&extClientUpdateFile, "file", "", "Filepath of updated external client definition in JSON")
	extClientUpdateCmd.Flags().StringVar(&description, "desc", "", "Description of the external client")
	extClientUpdateCmd.Flags().StringVar(&privateKey, "private_key", "", "Filepath of updated external client definition in JSON")
	extClientUpdateCmd.Flags().StringVar(&publicKey, "public_key", "", "Filepath of updated external client definition in JSON")
	extClientUpdateCmd.Flags().StringVar(&address, "ipv4_addr", "", "IPv4 address of the external client")
	extClientUpdateCmd.Flags().StringVar(&address6, "ipv6_addr", "", "IPv6 address of the external client")
	extClientUpdateCmd.Flags().StringVar(&ingressGatewayID, "ingress_gateway_id", "", "ID of the ingress gateway")
	extClientUpdateCmd.Flags().StringVar(&ingressGatewayEndpoint, "ingress_gateway_endpoint", "", "Endpoint of the ingress gateway")
	extClientUpdateCmd.Flags().StringVar(&ownerID, "owner_id", "", "External Client owner's ID")
	rootCmd.AddCommand(extClientUpdateCmd)
}
