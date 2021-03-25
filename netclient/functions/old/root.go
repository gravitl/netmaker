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
  "github.com/spf13/cobra"
  "context"
  homedir "github.com/mitchellh/go-homedir"
  "github.com/spf13/viper"
  "time"
  "log"
  nodepb "github.com/gravitl/netmaker/grpc"
  "google.golang.org/grpc"

)


var cfgFile string


var client nodepb.NodeServiceClient
var requestCtx context.Context
var requestOpts grpc.DialOption

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
  Use:   "grpc-test",
  Short: "A test cli which may or may not turn into a real one",
  Long: `huh. this isn't very long at all, is it?`,
  // Uncomment the following line if your bare application
  // has an action associated with it:
  //	Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
  if err := rootCmd.Execute(); err != nil {
    fmt.Println(err)
    os.Exit(1)
  }
}

func init() {
  cobra.OnInitialize(initConfig)

  // Here you will define your flags and configuration settings.
  // Cobra supports persistent flags, which, if defined here,
  // will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.blogclient.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

  fmt.Println("Starting Node Service Client")

  // Establish context to timeout after 10 seconds if server does not respond
  requestCtx, _ = context.WithTimeout(context.Background(), 10*time.Second)
  // Establish insecure grpc options (no TLS)
  requestOpts = grpc.WithInsecure()
  // Dial the server, returns a client connection
  conn, err := grpc.Dial("localhost:50051", requestOpts)
  if err != nil {
	log.Fatalf("Unable to establish client connection to localhost:50051: %v", err)
  }
  // Instantiate the BlogServiceClient with our client connection to the server
  client = nodepb.NewNodeServiceClient(conn)
}


// initConfig reads in config file and ENV variables if set.
func initConfig() {
  if cfgFile != "" {
    // Use config file from the flag.
    viper.SetConfigFile(cfgFile)
  } else {
    // Find home directory.
    home, err := homedir.Dir()
    if err != nil {
      fmt.Println(err)
      os.Exit(1)
    }

    // Search config in home directory with name ".grpc-test" (without extension).
    viper.AddConfigPath(home)
    viper.SetConfigName(".grpc-test")
  }

  viper.AutomaticEnv() // read in environment variables that match

  // If a config file is found, read it in.
  if err := viper.ReadInConfig(); err == nil {
    fmt.Println("Using config file:", viper.ConfigFileUsed())
  }
}

