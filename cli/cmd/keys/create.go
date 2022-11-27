package keys

import (
	"log"
	"strconv"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/spf13/cobra"
)

var keyName string

var keysCreateCmd = &cobra.Command{
	Use:   "create [NETWORK NAME] [NUM USES]",
	Args:  cobra.ExactArgs(2),
	Short: "Create an access key",
	Long:  `Create an access key`,
	Run: func(cmd *cobra.Command, args []string) {
		keyUses, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			log.Fatal(err)
		}
		key := &models.AccessKey{Uses: int(keyUses)}
		if keyName != "" {
			key.Name = keyName
		}
		functions.PrettyPrint(functions.CreateKey(args[0], key))
	},
}

func init() {
	keysCreateCmd.Flags().StringVar(&keyName, "name", "", "Name of the key")
	rootCmd.AddCommand(keysCreateCmd)
}
