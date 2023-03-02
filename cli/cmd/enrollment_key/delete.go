package enrollment_key

import (
	"fmt"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var enrollmentKeyDeleteCmd = &cobra.Command{
	Use:   "delete keyID",
	Args:  cobra.ExactArgs(1),
	Short: "Delete an enrollment key",
	Long:  `Delete an enrollment key`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.DeleteEnrollmentKey(args[0])
		fmt.Println("Enrollment key ", args[0], " deleted")
	},
}

func init() {
	rootCmd.AddCommand(enrollmentKeyDeleteCmd)
}
