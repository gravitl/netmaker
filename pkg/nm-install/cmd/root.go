/*
Copyright © 2023 Netmaker Team

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
	"math/rand"
	"os"

	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	pro          bool
	domain       string
	masterkey    string
	license      string
	tenantID     string
	email        string
	mqUsername   string
	mqPassword   string
	turnUsername string
	turnPassword string
	latest       string
	token        string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "nm-install",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print("\n\n")
		pterm.DefaultBigText.WithLetters(
			putils.LettersFromStringWithStyle("NETMAKER", pterm.FgCyan.ToStyle())).Render()
		getBuildType(&pro)
		setInstallVars()
		installDependencies()
		installNetmaker()
		installNmctl()
		createNetwork()
		installNetclient()
		pterm.Println("\nNetmaker setup is now complete. You are ready to begin using Netmaker")
		pterm.Println("Visit https://dashboard." + domain + " to log in")
	},
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
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().BoolVarP(&pro, "pro", "p", false, "install pro version")
	rootCmd.PersistentFlags().StringVarP(&domain, "domain", "d", "", "custom domain to use")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.BindPFlags(rootCmd.Flags())
	viper.AutomaticEnv() // read in environment variables that match
}

func getBuildType(pro *bool) {
	if !*pro {
		pterm.Println("Would you like to install Netmaker Community Edition (CE), or Netmaker Enterprise Edition(pro)")
		pterm.Print("\nPro will require you to create an account at https://app.netmaker.io\n\n")
		selection := getInput([]string{"Community Edition", "Enterprise Edition (pro)"})
		if selection == "Enterprise Edition (pro)" {
			*pro = true
		}
	}
}

func getInput(options []string) string {
	selected, err := pterm.DefaultInteractiveSelect.WithOptions(options).Show()
	if err != nil {
		panic(err)
	}
	return selected
}

func randomString(n int) string {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"
	result := make([]byte, n)
	for i := 0; i < n; i++ {
		result[i] = letters[rand.Intn(len(letters))]
	}
	return string(result)
}
