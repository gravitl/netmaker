package node

import (
	"os"
	"strconv"

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
		table.SetHeader([]string{"ID", "Addresses", "Network", "Egress", "Ingress", "Relay"})
		for _, d := range data {
			addresses := ""
			if d.Address.String() != "" {
				addresses += d.Address.String()
			}
			if d.Address6.String() != "" {
				if d.Address.String() != "" {
					addresses += ", "
				}
				addresses += d.Address6.String()
			}
			table.Append([]string{d.ID.String(), addresses, d.Network,
				strconv.FormatBool(d.IsEgressGateway), strconv.FormatBool(d.IsIngressGateway), strconv.FormatBool(d.IsRelay)})
		}
		table.Render()
	},
}

func init() {
	nodeListCmd.Flags().StringVar(&networkName, "network", "", "Network name specifier")
	rootCmd.AddCommand(nodeListCmd)
}
