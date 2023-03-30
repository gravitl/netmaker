package enrollment_key

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var enrollmentKeyListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.NoArgs,
	Short: "List enrollment keys",
	Long:  `List enrollment keys`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(functions.GetEnrollmentKeys())
	},
}

func init() {
	rootCmd.AddCommand(enrollmentKeyListCmd)
}
