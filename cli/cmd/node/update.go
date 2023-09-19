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
			node        = &models.ApiNode{}
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
			node.Address = address
			node.Address6 = address6
			node.LocalAddress = localAddress
			if relayedNodes != "" {
				node.RelayedNodes = strings.Split(relayedNodes, ",")
			}
			if egressGatewayRanges != "" {
				node.EgressGatewayRanges = strings.Split(egressGatewayRanges, ",")
			}
			node.ExpirationDateTime = int64(expirationDateTime)
			if defaultACL {
				node.DefaultACL = "yes"
			}
			node.DNSOn = dnsOn
			node.Connected = !disconnect
		}
		node.HostID = functions.GetNodeByID(networkName, nodeID).Host.ID.String()
		functions.PrettyPrint(functions.UpdateNode(networkName, nodeID, node))
	},
}

func init() {
	nodeUpdateCmd.Flags().StringVar(&nodeDefinitionFilePath, "file", "", "Filepath of updated node definition in JSON")
	nodeUpdateCmd.Flags().StringVar(&address, "ipv4_addr", "", "IPv4 address of the node")
	nodeUpdateCmd.Flags().StringVar(&address6, "ipv6_addr", "", "IPv6 address of the node")
	nodeUpdateCmd.Flags().StringVar(&localAddress, "local_addr", "", "Locally reachable address of the node")
	nodeUpdateCmd.Flags().StringVar(&name, "name", "", "Node name")
	nodeUpdateCmd.Flags().StringVar(&postUp, "post_up", "", "Commands to run after node is up `;` separated")
	nodeUpdateCmd.Flags().StringVar(&postDown, "post_down", "", "Commands to run after node is down `;` separated")
	nodeUpdateCmd.Flags().StringVar(&relayedNodes, "relayed_nodes", "", "relayed nodes if node acts as a relay")
	nodeUpdateCmd.Flags().StringVar(&egressGatewayRanges, "egress_addrs", "", "Addresses for egressing traffic if node acts as an egress")
	nodeUpdateCmd.Flags().IntVar(&expirationDateTime, "expiry", 0, "UNIX timestamp after which node will lose access to the network")
	nodeUpdateCmd.Flags().BoolVar(&defaultACL, "acl", false, "Enable default ACL ?")
	nodeUpdateCmd.Flags().BoolVar(&dnsOn, "dns", false, "Setup DNS entries for peers locally ?")
	nodeUpdateCmd.Flags().BoolVar(&disconnect, "disconnect", false, "Disconnect from the network ?")
	rootCmd.AddCommand(nodeUpdateCmd)
}
