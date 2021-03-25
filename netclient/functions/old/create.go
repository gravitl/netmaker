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
	"net/http"
	"io/ioutil"
        nodepb "github.com/gravitl/netmaker/grpc"
	"github.com/spf13/cobra"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new node",
	Long: `hi`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
    // Get the data from our flags 
		nodegroup, err := cmd.Flags().GetString("nodegroup")
		password, err := cmd.Flags().GetString("password")
		macaddress, err := cmd.Flags().GetString("macaddress")
		name, err := cmd.Flags().GetString("name")
		listenport, err := cmd.Flags().GetInt32("listenport")
		publickey, err := cmd.Flags().GetString("publickey")
		endpoint, err := cmd.Flags().GetString("endpoint")
		if err != nil {
			return err
		}
		if endpoint == "" {
			resp, err := http.Get("https://ifconfig.me")
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				bodyBytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			endpoint = string(bodyBytes)
			}
		}
    // Create a blog protobuffer message  
		node := &nodepb.Node{
			Password: password,
			Macaddress:    macaddress,
			Nodegroup:  nodegroup,
			Listenport: listenport,
			Publickey: publickey,
			Name: name,
			Endpoint: endpoint,
		}
    // RPC call
		res, err := client.CreateNode(
			context.TODO(),
      // wrap the blog message in a CreateBlog request message
			&nodepb.CreateNodeReq{
				Node: node,
			},
		)
		if err != nil {
			return err
		}
		fmt.Printf("Node created: %s\n", res.Node.Id)
		return err
	},
}

func init() {

	createCmd.Flags().StringP("name", "n", "", "The node name")
	createCmd.Flags().StringP("listenport", "l", "", "The wireguard port")
	createCmd.Flags().StringP("endpoint", "e", "", "The public IP")
	createCmd.Flags().StringP("macaddress", "m", "", "The local macaddress")
	createCmd.Flags().StringP("password", "p", "", "The password")
	createCmd.Flags().StringP("nodegroup", "g", "", "The group this will be added to")
	createCmd.Flags().StringP("publickey", "k", "", "The wireguard public key")
	createCmd.MarkFlagRequired("nodegroup")
	createCmd.MarkFlagRequired("password")
	createCmd.MarkFlagRequired("macaddress")
	rootCmd.AddCommand(createCmd)

}
