package enrollment_key

import (
	"strings"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/spf13/cobra"
)

var (
	expiration    int
	usesRemaining int
	networks      string
	unlimited     bool
	tags          string
)

var enrollmentKeyCreateCmd = &cobra.Command{
	Use:   "create [--args]",
	Args:  cobra.NoArgs,
	Short: "Create an enrollment key",
	Long:  `Create an enrollment key`,
	Run: func(cmd *cobra.Command, args []string) {
		enrollKey := &models.APIEnrollmentKey{
			Expiration:    int64(expiration),
			UsesRemaining: usesRemaining,
			Unlimited:     unlimited,
		}
		if networks != "" {
			enrollKey.Networks = strings.Split(networks, ",")
		}
		if tags != "" {
			enrollKey.Tags = strings.Split(tags, ",")
		}
		functions.PrettyPrint(functions.CreateEnrollmentKey(enrollKey))
	},
}

func init() {
	enrollmentKeyCreateCmd.Flags().IntVar(&expiration, "expiration", 0, "Expiration time of the key in UNIX timestamp format")
	enrollmentKeyCreateCmd.Flags().IntVar(&usesRemaining, "uses", 0, "Number of times this key can be used")
	enrollmentKeyCreateCmd.Flags().StringVar(&networks, "networks", "", "Comma-separated list of networks which the enrollment key can access")
	enrollmentKeyCreateCmd.Flags().BoolVar(&unlimited, "unlimited", false, "Should the key have unlimited uses ?")
	enrollmentKeyCreateCmd.Flags().StringVar(&tags, "tags", "", "Comma-separated list of any additional tags")
	rootCmd.AddCommand(enrollmentKeyCreateCmd)
}
