package network_user

import (
	"fmt"
	"strings"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models/promodels"
	"github.com/spf13/cobra"
)

var networkuserUpdateCmd = &cobra.Command{
	Use:   "update [NETWORK NAME]",
	Args:  cobra.ExactArgs(1),
	Short: "Update a network user",
	Long:  `Update a network user`,
	Run: func(cmd *cobra.Command, args []string) {
		user := &promodels.NetworkUser{
			AccessLevel: accessLevel,
			ClientLimit: clientLimit,
			NodeLimit:   nodeLimit, ID: promodels.NetworkUserID(id),
		}
		if clients != "" {
			user.Clients = strings.Split(clients, ",")
		}
		if nodes != "" {
			user.Nodes = strings.Split(nodes, ",")
		}
		functions.UpdateNetworkUser(args[0], user)
		fmt.Println("Success")
	},
}

func init() {
	networkuserUpdateCmd.Flags().IntVar(&accessLevel, "access_level", 0, "Custom access level")
	networkuserUpdateCmd.Flags().IntVar(&clientLimit, "client_limit", 0, "Maximum number of external clients that can be created")
	networkuserUpdateCmd.Flags().IntVar(&nodeLimit, "node_limit", 999999999, "Maximum number of nodes that can be attached to a network")
	networkuserUpdateCmd.Flags().StringVar(&clients, "clients", "", "Access to list of external clients (comma separated)")
	networkuserUpdateCmd.Flags().StringVar(&nodes, "nodes", "", "Access to list of nodes (comma separated)")
	networkuserUpdateCmd.Flags().StringVar(&id, "id", "", "ID of the network user")
	networkuserUpdateCmd.MarkFlagRequired("id")
	rootCmd.AddCommand(networkuserUpdateCmd)
}
