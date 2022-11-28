package dns

import (
	"fmt"
	"os"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/guumaster/tablewriter"
	"github.com/spf13/cobra"
)

var dnsListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.NoArgs,
	Short: "List DNS entries",
	Long:  `List DNS entries`,
	Run: func(cmd *cobra.Command, args []string) {
		var data []models.DNSEntry
		if networkName != "" {
			switch dnsType {
			case "node":
				data = *functions.GetNodeDNS(networkName)
			case "custom":
				data = *functions.GetCustomDNS(networkName)
			case "network", "":
				data = *functions.GetNetworkDNS(networkName)
			default:
				fmt.Println("Invalid DNS type provided ", dnsType)
			}
		} else {
			data = *functions.GetDNS()
		}
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Name", "Network", "IPv4 Address", "IPv6 Address"})
		for _, d := range data {
			table.Append([]string{d.Name, d.Network, d.Address, d.Address6})
		}
		table.Render()
	},
}

func init() {
	dnsListCmd.Flags().StringVar(&networkName, "network", "", "Network name")
	dnsListCmd.Flags().StringVar(&dnsType, "type", "", "Type of DNS records to fetch ENUM(node, custom, network)")
	rootCmd.AddCommand(dnsListCmd)
}
