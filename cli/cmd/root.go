package cmd

import (
	"os"

	"github.com/gravitl/netmaker/cli/cmd/acl"
	"github.com/gravitl/netmaker/cli/cmd/commons"
	"github.com/gravitl/netmaker/cli/cmd/context"
	"github.com/gravitl/netmaker/cli/cmd/dns"
	"github.com/gravitl/netmaker/cli/cmd/enrollment_key"
	"github.com/gravitl/netmaker/cli/cmd/ext_client"
	"github.com/gravitl/netmaker/cli/cmd/failover"
	"github.com/gravitl/netmaker/cli/cmd/host"
	"github.com/gravitl/netmaker/cli/cmd/metrics"
	"github.com/gravitl/netmaker/cli/cmd/network"
	"github.com/gravitl/netmaker/cli/cmd/node"
	"github.com/gravitl/netmaker/cli/cmd/server"
	"github.com/gravitl/netmaker/cli/cmd/user"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "nmctl",
	Short: "CLI for interacting with Netmaker Server",
	Long:  `CLI for interacting with Netmaker Server`,
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
	rootCmd.PersistentFlags().StringVarP(&commons.OutputFormat, "output", "o", "", "List output in specific format (Enum:- json)")
	// Bind subcommands here
	rootCmd.AddCommand(network.GetRoot())
	rootCmd.AddCommand(context.GetRoot())
	rootCmd.AddCommand(acl.GetRoot())
	rootCmd.AddCommand(node.GetRoot())
	rootCmd.AddCommand(dns.GetRoot())
	rootCmd.AddCommand(server.GetRoot())
	rootCmd.AddCommand(ext_client.GetRoot())
	rootCmd.AddCommand(user.GetRoot())
	rootCmd.AddCommand(metrics.GetRoot())
	rootCmd.AddCommand(host.GetRoot())
	rootCmd.AddCommand(enrollment_key.GetRoot())
	rootCmd.AddCommand(failover.GetRoot())
}
