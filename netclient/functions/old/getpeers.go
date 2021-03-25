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
        nodepb "github.com/gravitl/netmaker/grpc"
	"context"
	"io"
	"github.com/spf13/cobra"
        "github.com/gravitl/netmaker/wcagent/functions"
        "google.golang.org/grpc/metadata"
        "google.golang.org/grpc"

)

// getpeersCmd represents the getpeers command
var getpeersCmd = &cobra.Command{
	Use:   "getpeers",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	SilenceUsage: true,
        RunE: func(cmd *cobra.Command, args []string) error {
                fmt.Println("read called")
                group, err := cmd.Flags().GetString("group")
                if err != nil {
                        return err
                }
                req := &nodepb.GetPeersReq{
                        Group: group,
                }
                ctx := context.Background()
                ctx, err = functions.SetJWT(client)
                if err != nil {
                        return err
                }

                var header metadata.MD

                stream, err := client.GetPeers(ctx, req, grpc.Header(&header))
                if err != nil {
                        return err
                }
                //fmt.Println(res)

		for {
			// stream.Recv returns a pointer to a ListBlogRes at the current iteration
			res, err := stream.Recv()
			// If end of stream, break the loop
			if err == io.EOF {
				break
			}
			// if err, return an error
			if err != nil {
				return err
			}
			// If everything went well use the generated getter to print the blog message
			fmt.Println(res.Peers)
		}

                return nil
        },
}

func init() {
        getpeersCmd.Flags().StringP("group", "g", "", "The group of the node")
        getpeersCmd.MarkFlagRequired("group")
	rootCmd.AddCommand(getpeersCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// getpeersCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// getpeersCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
