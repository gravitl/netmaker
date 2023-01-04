package network_user

import (
	"fmt"
	"strings"

	"github.com/gravitl/netmaker/cli/functions"
	"github.com/gravitl/netmaker/models/promodels"
	"github.com/spf13/cobra"
)

var networkuserCreateCmd = &cobra.Command{
	Use:   "create [NETWORK NAME]",
	Args:  cobra.ExactArgs(1),
	Short: "Create a network user",
	Long:  `Create a network user`,
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
		functions.CreateNetworkUser(args[0], user)
		fmt.Println("Success")
	},
}

func init() {
	networkuserCreateCmd.Flags().IntVar(&accessLevel, "access_level", 0, "Custom access level")
	networkuserCreateCmd.Flags().IntVar(&clientLimit, "client_limit", 0, "Maximum number of external clients that can be created")
	networkuserCreateCmd.Flags().IntVar(&nodeLimit, "node_limit", 999999999, "Maximum number of nodes that can be attached to a network")
	networkuserCreateCmd.Flags().StringVar(&clients, "clients", "", "Access to list of external clients (comma separated)")
	networkuserCreateCmd.Flags().StringVar(&nodes, "nodes", "", "Access to list of nodes (comma separated)")
	networkuserCreateCmd.Flags().StringVar(&id, "id", "", "ID of the network user")
	networkuserCreateCmd.MarkFlagRequired("id")
	rootCmd.AddCommand(networkuserCreateCmd)
}
