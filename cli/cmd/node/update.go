package node

import (
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models"
	"github.com/spf13/cobra"
)

var nodeUpdateCmd = &cobra.Command{
	Use:   "update [NETWORK NAME] [NODE ID]",
	Args:  cobra.ExactArgs(2),
	Short: "Update a Node",
	Long:  `Update a Node`,
	Run: func(cmd *cobra.Command, args []string) {
		var (
			node        = &models.Node{}
			networkName = args[0]
			nodeID      = args[1]
		)
		if nodeDefinitionFilePath != "" {
			content, err := os.ReadFile(nodeDefinitionFilePath)
			if err != nil {
				log.Fatal("Error when opening file: ", err)
			}
			if err := json.Unmarshal(content, node); err != nil {
				log.Fatal(err)
			}
		} else {
			if endpoint != "" {
				node.Endpoint = endpoint
				node.IsStatic = "no"
			}
			node.ListenPort = int32(listenPort)
			node.Address = address
			node.Address6 = address6
			node.LocalAddress = localAddress
			node.Name = name
			node.PostUp = postUp
			node.PostDown = postDown
			if allowedIPs != "" {
				node.AllowedIPs = strings.Split(allowedIPs, ",")
			}
			node.PersistentKeepalive = int32(keepAlive)
			if relayAddrs != "" {
				node.RelayAddrs = strings.Split(relayAddrs, ",")
			}
			if egressGatewayRanges != "" {
				node.EgressGatewayRanges = strings.Split(egressGatewayRanges, ",")
			}
			if localRange != "" {
				node.LocalRange = localRange
				node.IsLocal = "yes"
			}
			node.MTU = int32(mtu)
			node.ExpirationDateTime = int64(expirationDateTime)
			if defaultACL {
				node.DefaultACL = "yes"
			}
			if dnsOn {
				node.DNSOn = "yes"
			}
			if disconnect {
				node.Connected = "no"
			}
			if networkHub {
				node.IsHub = "yes"
			}
		}
		functions.PrettyPrint(functions.UpdateNode(networkName, nodeID, node))
	},
}

func init() {
	nodeUpdateCmd.Flags().StringVar(&nodeDefinitionFilePath, "file", "", "Filepath of updated node definition in JSON")
	nodeUpdateCmd.Flags().StringVar(&endpoint, "endpoint", "", "Public endpoint of the node")
	nodeUpdateCmd.Flags().IntVar(&listenPort, "listen_port", 0, "Default wireguard port for the node")
	nodeUpdateCmd.Flags().StringVar(&address, "ipv4_addr", "", "IPv4 address of the node")
	nodeUpdateCmd.Flags().StringVar(&address6, "ipv6_addr", "", "IPv6 address of the node")
	nodeUpdateCmd.Flags().StringVar(&localAddress, "local_addr", "", "Locally reachable address of the node")
	nodeUpdateCmd.Flags().StringVar(&name, "name", "", "Node name")
	nodeUpdateCmd.Flags().StringVar(&postUp, "post_up", "", "Commands to run after node is up `;` separated")
	nodeUpdateCmd.Flags().StringVar(&postDown, "post_down", "", "Commands to run after node is down `;` separated")
	nodeUpdateCmd.Flags().StringVar(&allowedIPs, "allowed_addrs", "", "Additional private addresses given to the node (comma separated)")
	nodeUpdateCmd.Flags().IntVar(&keepAlive, "keep_alive", 0, "Interval in which packets are sent to keep connections open with peers")
	nodeUpdateCmd.Flags().StringVar(&relayAddrs, "relay_addrs", "", "Addresses for relaying connections if node acts as a relay")
	nodeUpdateCmd.Flags().StringVar(&egressGatewayRanges, "egress_addrs", "", "Addresses for egressing traffic if node acts as an egress")
	nodeUpdateCmd.Flags().StringVar(&localRange, "local_range", "", "Local range in which the node will look for private addresses to use as an endpoint if `LocalNetwork` is enabled")
	nodeUpdateCmd.Flags().IntVar(&mtu, "mtu", 0, "MTU size")
	nodeUpdateCmd.Flags().IntVar(&expirationDateTime, "expiry", 0, "UNIX timestamp after which node will lose access to the network")
	nodeUpdateCmd.Flags().BoolVar(&defaultACL, "acl", false, "Enable default ACL ?")
	nodeUpdateCmd.Flags().BoolVar(&dnsOn, "dns", false, "Setup DNS entries for peers locally ?")
	nodeUpdateCmd.Flags().BoolVar(&disconnect, "disconnect", false, "Disconnect from the network ?")
	nodeUpdateCmd.Flags().BoolVar(&networkHub, "hub", false, "On a point to site network, this node is the only one which all peers connect to ?")
	rootCmd.AddCommand(nodeUpdateCmd)
}
