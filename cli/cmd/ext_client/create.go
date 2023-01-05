package ext_client

import (
	"fmt"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var extClientID string

var extClientCreateCmd = &cobra.Command{
	Use:   "create [NETWORK NAME] [NODE ID]",
	Args:  cobra.ExactArgs(2),
	Short: "Create an External Client",
	Long:  `Create an External Client`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.CreateExtClient(args[0], args[1], extClientID)
		fmt.Println("Success")
	},
}

func init() {
	extClientCreateCmd.Flags().StringVar(&extClientID, "id", "", "ID of the external client")
	rootCmd.AddCommand(extClientCreateCmd)
}
