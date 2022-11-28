package ext_client

import (
	"os"
	"strconv"
	"time"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/guumaster/tablewriter"
	"github.com/spf13/cobra"
)

var networkName string

var extClientListCmd = &cobra.Command{
	Use:   "list",
	Args:  cobra.NoArgs,
	Short: "List External Clients",
	Long:  `List External Clients`,
	Run: func(cmd *cobra.Command, args []string) {
		var data []models.ExtClient
		if networkName != "" {
			data = *functions.GetNetworkExtClients(networkName)
		} else {
			data = *functions.GetAllExtClients()
		}
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"Client ID", "Network", "IPv4 Address", "IPv6 Address", "Enabled", "Last Modified"})
		for _, d := range data {
			table.Append([]string{d.ClientID, d.Network, d.Address, d.Address6, strconv.FormatBool(d.Enabled), time.Unix(d.LastModified, 0).String()})
		}
		table.Render()
	},
}

func init() {
	extClientListCmd.Flags().StringVar(&networkName, "network", "", "Network name")
	rootCmd.AddCommand(extClientListCmd)
}
