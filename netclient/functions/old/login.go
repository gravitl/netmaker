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
	"os"
	"encoding/json"
	"io/ioutil"
        nodepb "github.com/gravitl/netmaker/grpc"
	"context"
	"github.com/spf13/cobra"
)

// loginCmd represents the login command
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Get auth token",
	Long: `Get auth token`,
        SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
                password, err := cmd.Flags().GetString("password")
                macaddress, err := cmd.Flags().GetString("macaddress")
                if err != nil {
                        return err
                }
                home, err := os.UserHomeDir()
		data := SetConfiguration{
			MacAddress: macaddress,
			Password: password,
		}
		file, err := json.MarshalIndent(data, "", " ")
		err = ioutil.WriteFile(home + "/.wcconfig", file, 0644)
		if err != nil {
                        return err
                }
                login := &nodepb.LoginRequest{
                        Password: password,
                        Macaddress:    macaddress,
                }
    // RPC call
                res, err := client.Login(context.TODO(), login)
                if err != nil {
                        return err
                }
                fmt.Printf("Token: %s\n", res.Accesstoken)
		tokenstring := []byte(res.Accesstoken)
		err = ioutil.WriteFile(home + "/.wctoken", tokenstring, 0644)
		if err != nil {
			return err
		}
                return err

	},
}

func init() {

        loginCmd.Flags().StringP("macaddress", "m", "", "The local macaddress")
        loginCmd.Flags().StringP("password", "p", "", "The password")

        loginCmd.MarkFlagRequired("password")
        loginCmd.MarkFlagRequired("macaddress")
	rootCmd.AddCommand(loginCmd)
}

type SetConfiguration struct {
        MacAddress string
        Password string
}
