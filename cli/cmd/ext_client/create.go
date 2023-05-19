package ext_client

import (
	"fmt"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/spf13/cobra"
)

var (
	extClientID string
	publicKey   string
	dns         string
	allowedips  []string
)

var extClientCreateCmd = &cobra.Command{
	Use:   "create [NETWORK NAME] [NODE ID]",
	Args:  cobra.ExactArgs(2),
	Short: "Create an External Client",
	Long:  `Create an External Client`,
	Run: func(cmd *cobra.Command, args []string) {
		extClient := models.CustomExtClient{
			ClientID:        extClientID,
			PublicKey:       publicKey,
			DNS:             dns,
			ExtraAllowedIPs: allowedips,
		}

		functions.CreateExtClient(args[0], args[1], extClient)
		fmt.Println("Success")
	},
}

func init() {
	extClientCreateCmd.Flags().StringVar(&extClientID, "id", "", "ID of the external client")
	extClientCreateCmd.Flags().StringVar(&publicKey, "public_key", "", "updated public key of the external client")
	extClientCreateCmd.Flags().StringVar(&dns, "dns", "", "updated DNS of the external client")
	extClientCreateCmd.Flags().StringSliceVar(&allowedips, "allowedips", []string{}, "updated extra allowed IPs of the external client")
	rootCmd.AddCommand(extClientCreateCmd)
}
