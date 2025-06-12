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
			table.SetHeader([]string{"ID", "Addresses", "Network", "Egress", "Remote Access Gateway", "Relay", "Type"})
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
				network := d.Network
				id := d.ID
				nodeType := "Device"

				if d.IsStatic {
					id = d.StaticNode.ClientID
					nodeType = "Static"
				}
				if d.IsUserNode {
					id = d.StaticNode.OwnerID
					nodeType = "User"
				}
				if d.IsStatic || d.IsUserNode {
					addresses = d.StaticNode.Address
					if d.StaticNode.Address6 != "" {
						if addresses != "" {
							addresses += ", "
						}
						addresses += d.StaticNode.Address6
					}
					network = d.StaticNode.Network
				}

				table.Append([]string{id, addresses, network,
					strconv.FormatBool(d.IsEgressGateway), strconv.FormatBool(d.IsIngressGateway), strconv.FormatBool(d.IsRelay), nodeType})
			}
			table.Render()
		}
	},
}

func init() {
	nodeListCmd.Flags().StringVar(&networkName, "network", "", "Network name specifier")
	rootCmd.AddCommand(nodeListCmd)
}
