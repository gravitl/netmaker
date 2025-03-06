package gateway

import (
	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/spf13/cobra"
	"strings"
)

var externalClientDNS string
var isInternetGateway bool
var metadata string
var persistentKeepAlive uint
var mtu uint

var gatewayCreateCmd = &cobra.Command{
	Use:   "create [NETWORK NAME] [NODE ID] [RELAYED NODES ID (comma separated)]",
	Args:  cobra.ExactArgs(3),
	Short: "Create a new Gateway on a Netmaker network.",
	Long: `
Configures a node as a gateway in a specified network, allowing it to relay traffic for other nodes. The gateway can also function as an internet gateway if specified.

Arguments:
NETWORK NAME:		The name of the network where the gateway will be created.
NODE ID:			The ID of the node to be configured as a gateway.
RELAYED NODES ID:	A comma-separated list of node IDs that will be relayed through this gateway.
`,
	Run: func(cmd *cobra.Command, args []string) {
		functions.PrettyPrint(
			functions.CreateGateway(
				models.IngressRequest{
					ExtclientDNS:        externalClientDNS,
					IsInternetGateway:   isInternetGateway,
					Metadata:            metadata,
					PersistentKeepalive: int32(persistentKeepAlive),
					MTU:                 int32(mtu),
				},
				models.RelayRequest{
					NodeID:       args[0],
					NetID:        args[1],
					RelayedNodes: strings.Split(args[2], ","),
				},
			),
		)
	},
}

func init() {
	gatewayCreateCmd.Flags().StringVarP(&externalClientDNS, "dns", "d", "", "the IP address of the DNS server to be used by external clients")
	gatewayCreateCmd.Flags().BoolVarP(&isInternetGateway, "internet", "i", false, "if set, the gateway will route traffic to the internet")
	gatewayCreateCmd.Flags().StringVarP(&metadata, "note", "n", "", "description or metadata to be associated with the gateway")
	gatewayCreateCmd.Flags().UintVarP(&persistentKeepAlive, "keep-alive", "k", 20, "the keep-alive interval (in seconds) for maintaining persistent connections")
	gatewayCreateCmd.Flags().UintVarP(&mtu, "mtu", "m", 1420, "the maximum transmission unit (MTU) size in bytes")
	rootCmd.AddCommand(gatewayCreateCmd)
}
