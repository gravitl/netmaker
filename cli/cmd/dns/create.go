package dns

import (
	"log"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/spf13/cobra"
)

var dnsCreateCmd = &cobra.Command{
	Use:   "create",
	Args:  cobra.NoArgs,
	Short: "Create a DNS entry",
	Long:  `Create a DNS entry`,
	Run: func(cmd *cobra.Command, args []string) {
		if address == "" && address6 == "" {
			log.Fatal("Either IPv4 or IPv6 address is required")
		}
		dnsEntry := &models.DNSEntry{Name: dnsName, Address: address, Address6: address6, Network: networkName}
		functions.PrettyPrint(functions.CreateDNS(networkName, dnsEntry))
	},
}

func init() {
	dnsCreateCmd.Flags().StringVar(&dnsName, "name", "", "Name of the DNS entry")
	dnsCreateCmd.MarkFlagRequired("name")
	dnsCreateCmd.Flags().StringVar(&networkName, "network", "", "Name of the Network")
	dnsCreateCmd.MarkFlagRequired("network")
	dnsCreateCmd.Flags().StringVar(&address, "ipv4_addr", "", "IPv4 Address")
	dnsCreateCmd.Flags().StringVar(&address6, "ipv6_addr", "", "IPv6 Address")
	rootCmd.AddCommand(dnsCreateCmd)
}
