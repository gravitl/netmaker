package dns

import (
	"fmt"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var dnsPushCmd = &cobra.Command{
	Use:   "push",
	Args:  cobra.NoArgs,
	Short: "Push latest DNS entries",
	Long:  `Push latest DNS entries`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(*functions.PushDNS())
	},
}

func init() {
	rootCmd.AddCommand(dnsPushCmd)
}
