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
        "github.com/gravitl/netmaker/wcagent/functions"
	nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/spf13/cobra"
        "google.golang.org/grpc/metadata"
        "google.golang.org/grpc"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a Nodeby its MAC",
	Long: `Delete a node post by it's macaddress in mongodb.
	
	If no node is found with the MAC it will return a 'Not Found' error`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		macaddress, err := cmd.Flags().GetString("macaddress")
		if err != nil {
			return err
		}
		req := &nodepb.DeleteNodeReq{
			Macaddress: macaddress,
		}
                ctx := context.Background()
                ctx, err = functions.SetJWT(client)
                if err != nil {
                        return err
                }

                var header metadata.MD

                _, err = client.DeleteNode(ctx, req, grpc.Header(&header))
		if err != nil {
			return err
		}
		fmt.Printf("Succesfully deleted the node with macaddress %s\n", macaddress)
		return nil
	},
}

func init() {
	deleteCmd.Flags().StringP("macaddress", "m", "", "The macaddress of the node")
	deleteCmd.MarkFlagRequired("macaddress")
	rootCmd.AddCommand(deleteCmd)
}


