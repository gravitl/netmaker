package dns

import (
	"fmt"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/spf13/cobra"
)

var dnsListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.NoArgs,
	Short: "List DNS entries",
	Long:  `List DNS entries`,
	Run: func(cmd *cobra.Command, args []string) {
		if networkName != "" {
			switch dnsType {
			case "node":
				functions.PrettyPrint(functions.GetNodeDNS(networkName))
			case "custom":
				functions.PrettyPrint(functions.GetCustomDNS(networkName))
			case "network", "":
				functions.PrettyPrint(functions.GetNetworkDNS(networkName))
			default:
				fmt.Println("Invalid DNS type provided ", dnsType)
			}
		} else {
			functions.PrettyPrint(functions.GetDNS())
		}
	},
}

func init() {
	dnsListCmd.Flags().StringVar(&networkName, "network", "", "Network name")
	dnsListCmd.Flags().StringVar(&dnsType, "type", "", "Type of DNS records to fetch ENUM(node, custom, network)")
	rootCmd.AddCommand(dnsListCmd)
}
