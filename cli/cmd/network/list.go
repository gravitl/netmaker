package network

import (
	"os"
	"time"

	"github.com/gravitl/netmaker/cli/cmd/commons"
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/guumaster/tablewriter"
	"github.com/spf13/cobra"
)

var networkListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all Networks",
	Long:  `List all Networks`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		networks := functions.GetNetworks()
		switch commons.OutputFormat {
		case commons.JsonOutput:
			functions.PrettyPrint(networks)
		default:
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"NetId", "Address Range (IPv4)", "Address Range (IPv6)", "Network Last Modified", "Nodes Last Modified"})
			for _, n := range *networks {
				networkLastModified := n.UpdatedAt.Format(time.RFC3339)
				nodesLastModified := n.NodesUpdatedAt.Format(time.RFC3339)
				table.Append([]string{n.Name, n.AddressRange, n.AddressRange6, networkLastModified, nodesLastModified})
			}
			table.Render()
		}
	},
}

func init() {
	rootCmd.AddCommand(networkListCmd)
}
