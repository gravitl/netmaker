package node

import (
	"os"
	"strconv"

	"github.com/gravitl/netmaker/cli/cmd/commons"
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
		var data []models.ApiNode
		if networkName != "" {
			data = *functions.GetNodes(networkName)
		} else {
			data = *functions.GetNodes()
		}
		switch commons.OutputFormat {
		case commons.JsonOutput:
			functions.PrettyPrint(data)
		default:
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Addresses", "Network", "Egress", "Remote Access Gateway", "Relay"})
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
				table.Append([]string{d.ID, addresses, d.Network,
					strconv.FormatBool(d.IsEgressGateway), strconv.FormatBool(d.IsIngressGateway), strconv.FormatBool(d.IsRelay)})
			}
			table.Render()
		}
	},
}

func init() {
	nodeListCmd.Flags().StringVar(&networkName, "network", "", "Network name specifier")
	rootCmd.AddCommand(nodeListCmd)
}
