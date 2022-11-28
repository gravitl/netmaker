package node

import (
	"os"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/guumaster/tablewriter"
	"github.com/spf13/cobra"
)

// nodeListCmd lists all nodes
var nodeListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.NoArgs,
	Short: "List all nodes",
	Long:  `List all nodes`,
	Run: func(cmd *cobra.Command, args []string) {
		var data []models.Node
		if networkName != "" {
			data = *functions.GetNodes(networkName)
		} else {
			data = *functions.GetNodes()
		}
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Name", "Addresses", "Version", "Network", "Egress", "Ingress", "Relay", "ID"})
		for _, d := range data {
			addresses := ""
			if d.Address != "" {
				addresses += d.Address
			}
			if d.Address6 != "" {
				if d.Address != "" {
					addresses += ", "
				}
				addresses += d.Address6
			}
			table.Append([]string{d.Name, addresses, d.Version, d.Network, d.IsEgressGateway, d.IsIngressGateway, d.IsRelay, d.ID})
		}
		table.Render()
	},
}

func init() {
	nodeListCmd.Flags().StringVar(&networkName, "network", "", "Network name specifier")
	rootCmd.AddCommand(nodeListCmd)
}
