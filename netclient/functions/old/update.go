/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
        "context"
        nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/spf13/cobra"
        "github.com/gravitl/netmaker/wcagent/functions"
        "google.golang.org/grpc/metadata"
        "google.golang.org/grpc"

)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Find a Node by its Macaddress",
	Long: `Find a node by it's mongoDB Unique macaddressentifier.

	If no node is found for the Macaddress it will return a 'Not Found' error`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get the flags from CLI
                nodegroup, err := cmd.Flags().GetString("nodegroup")
                password, err := cmd.Flags().GetString("password")
                macaddress, err := cmd.Flags().GetString("macaddress")
                name, err := cmd.Flags().GetString("name")
                listenport, err := cmd.Flags().GetInt32("listenport")
                publickey, err := cmd.Flags().GetString("publickey")
                endpoint, err := cmd.Flags().GetString("endpoint")
		// Create an UpdateNodeRequest
		node := &nodepb.Node{
				Password: password,
				Macaddress:    macaddress,
				Nodegroup:  nodegroup,
				Listenport: listenport,
				Publickey: publickey,
				Name: name,
				Endpoint: endpoint,
			}
		req := &nodepb.UpdateNodeReq{
				Node: node,
			}
                ctx := context.Background()
                ctx, err = functions.SetJWT(client)
                if err != nil {
                        return err
                }

                var header metadata.MD

		res, err := client.UpdateNode(ctx, req, grpc.Header(&header))
		if err != nil {
			return err
		}

		fmt.Println(res)
		return nil
	},
}

func init() {
        updateCmd.Flags().StringP("name", "n", "", "The node name")
        updateCmd.Flags().StringP("listenport", "l", "", "The wireguard port")
        updateCmd.Flags().StringP("endpoint", "e", "", "The public IP")
        updateCmd.Flags().StringP("macaddress", "m", "", "The local macaddress")
        updateCmd.Flags().StringP("password", "p", "", "The password")
        updateCmd.Flags().StringP("nodegroup", "g", "", "The group this will be added to")
        updateCmd.Flags().StringP("publickey", "k", "", "The wireguard public key")
        updateCmd.MarkFlagRequired("nodegroup")
        updateCmd.MarkFlagRequired("password")
        updateCmd.MarkFlagRequired("macaddress")
	rootCmd.AddCommand(updateCmd)
}
