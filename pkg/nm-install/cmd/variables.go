package cmd

import (
	"os"
	"strings"

	"github.com/bitfield/script"
	"github.com/pterm/pterm"
	"github.com/rdegges/go-ipify"
)

func setInstallVars() {
	askDomain := false
	ip, err := ipify.GetIp()
	if err != nil {
		panic(err)
	}
	ipDash := strings.ReplaceAll(ip, ".", "-")
	if domain == "" {
		domain = "nm." + ipDash + ".nip.io"
		askDomain = true
	}
	if masterkey == "" {
		masterkey = randomString(32)
	}
	pterm.Println("masterkey set: ", masterkey)
	mqUsername = "netmaker"
	mqPassword = randomString(32)
	pterm.Println("mq creditials:", mqUsername, mqPassword)
	turnUsername = "netmaker"
	turnPassword = randomString(32)
	pterm.Println("turn creditials:", turnUsername, turnPassword)
	if askDomain {
		pterm.Println("\nWould you like to use your own domain for netmaker, or an auto-generated domain?")
		pterm.Println("\nTo use your own domain, add a Wildcard DNS record (e.x: *.netmaker.example.com) pointing to", ip)
		pterm.Print("\nIMPORTANT: Due to the high volume of requests, the auto-generated domain has been rate-limited by the certificate provider.")
		pterm.Print("For this reason, we ", pterm.LightMagenta("STRONGLY RECOMMEND"), " using your own domain. Using the auto-generated domain may lead to a failed installation due to rate limiting.\n\n")
		domainType := getInput([]string{"Auto Generated " + domain, "Custom Domain (e.g. netmaker.example.com)"})
		if strings.Contains(domainType, "Custom") {
			script.Echo("Enter Custom Domain (ensure *.domain points to " + ip).Stdout()
			domain, err = pterm.DefaultInteractiveTextInput.WithMultiLine(false).Show()
			if err != nil {
				panic(err)
			}
		}
	}
	pterm.Print("\nThe following subdomains will be used:\n\n")
	pterm.Printf("dashboard.%s\n", domain)
	pterm.Printf("api.%s\n", domain)
	pterm.Printf("broker.%s\n", domain)
	pterm.Printf("turn.%s\n", domain)
	pterm.Printf("turnapi.%s\n", domain)
	if pro {
		pterm.Printf("prometheus.%s\n", domain)
		pterm.Printf("netmaker-exporter.%s\n", domain)
		pterm.Printf("grafana.%s\n", domain)
	}
	pterm.Print("\n\n")
	ok, err := pterm.DefaultInteractiveConfirm.WithDefaultValue(true).Show()
	if err != nil {
		panic(err)
	}
	if !ok {
		os.Exit(1)
	}
	pterm.Print("\nEnter email address for certificate registration\n\n")
	email, err = pterm.DefaultInteractiveTextInput.WithMultiLine(false).Show()
	if err != nil {
		panic(err)
	}
	if pro {
		pterm.Println("\nProvide Details for Pro installation")
		pterm.Println("\t1. Log into https://app.netmaker.io")
		pterm.Println("\t2. follow instructions to get a license at: https://docs.netmaker.io/ee/ee-setup.html")
		pterm.Println("\t3. Retrieve License and Tenant ID")
		pterm.Println("\t4. note email address")
		license, err = pterm.DefaultInteractiveTextInput.WithMultiLine(false).WithDefaultText("Licence Key").Show()
		if err != nil {
			panic(err)
		}
		tenantID, err = pterm.DefaultInteractiveTextInput.WithMultiLine(false).WithDefaultText("Tenant ID").Show()
		if err != nil {
			panic(err)
		}
	}
}
