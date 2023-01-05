package network

import (
	"log"
	"strconv"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var networkNodeLimitCmd = &cobra.Command{
	Use:   "node_limit [NETWORK NAME] [NEW LIMIT]",
	Short: "Update network nodel limit",
	Long:  `Update network nodel limit`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		nodelimit, err := strconv.ParseInt(args[1], 10, 32)
		if err != nil {
			log.Fatal(err)
		}
		functions.PrettyPrint(functions.UpdateNetworkNodeLimit(args[0], int32(nodelimit)))
	},
}

func init() {
	rootCmd.AddCommand(networkNodeLimitCmd)
}
