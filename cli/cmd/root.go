package cmd

import (
	"os"

	"github.com/gravitl/netmaker/cli/cmd/acl"
	"github.com/gravitl/netmaker/cli/cmd/context"
	"github.com/gravitl/netmaker/cli/cmd/dns"
	"github.com/gravitl/netmaker/cli/cmd/ext_client"
	"github.com/gravitl/netmaker/cli/cmd/keys"
	"github.com/gravitl/netmaker/cli/cmd/metrics"
	"github.com/gravitl/netmaker/cli/cmd/network"
	"github.com/gravitl/netmaker/cli/cmd/network_user"
	"github.com/gravitl/netmaker/cli/cmd/node"
	"github.com/gravitl/netmaker/cli/cmd/server"
	"github.com/gravitl/netmaker/cli/cmd/user"
	"github.com/gravitl/netmaker/cli/cmd/usergroup"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "netmaker",
	Short: "CLI for interacting with Netmaker Server",
	Long:  `CLI for interacting with Netmaker Server`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// GetRoot returns the root of all subcommands
func GetRoot() *cobra.Command {
	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.tctl.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	// IMP: Bind subcommands here
	rootCmd.AddCommand(network.GetRoot())
	rootCmd.AddCommand(context.GetRoot())
	rootCmd.AddCommand(keys.GetRoot())
	rootCmd.AddCommand(acl.GetRoot())
	rootCmd.AddCommand(node.GetRoot())
	rootCmd.AddCommand(dns.GetRoot())
	rootCmd.AddCommand(server.GetRoot())
	rootCmd.AddCommand(ext_client.GetRoot())
	rootCmd.AddCommand(user.GetRoot())
	rootCmd.AddCommand(usergroup.GetRoot())
	rootCmd.AddCommand(metrics.GetRoot())
	rootCmd.AddCommand(network_user.GetRoot())
}
