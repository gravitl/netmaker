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
	"github.com/gravitl/netmaker/wcagent/functions"
	"github.com/spf13/cobra"
        "google.golang.org/grpc/metadata"
	"google.golang.org/grpc"
)

// readCmd represents the read command
var readCmd = &cobra.Command{
	Use:   "read",
	Short: "Find a Node by its Mac Address",
	Long: `Find a node by it's macaddress, stored in mongoDB.
	
	If no node is found with the corresponding MAC it will return a 'Not Found' error`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("read called")
		macaddress, err := cmd.Flags().GetString("macaddress")
                group, err := cmd.Flags().GetString("group")
		if err != nil {
			return err
		}
		req := &nodepb.ReadNodeReq{
			Macaddress: macaddress,
			Group: group,
		}
		ctx := context.Background()
		ctx, err = functions.SetJWT(client)
                if err != nil {
                        return err
                }

		var header metadata.MD

		res, err := client.ReadNode(ctx, req, grpc.Header(&header))
		if err != nil {
			return err
		}
		fmt.Println(res)
		return nil
	},
}

func init() {


	readCmd.Flags().StringP("macaddress", "m", "", "The macaddress of the node")
	readCmd.Flags().StringP("group", "g", "", "The group of the node")
	readCmd.MarkFlagRequired("macaddress")
	readCmd.MarkFlagRequired("group")
	rootCmd.AddCommand(readCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// readCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// readCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
