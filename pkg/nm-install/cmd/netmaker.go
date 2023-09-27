package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/bitfield/script"
	"github.com/joho/godotenv"
	"github.com/pterm/pterm"
)

type Release struct {
	Version string `json:"tag_name"`
}

func installNetmaker() {
	var netEnv map[string]string
	latest = getLatestRelease()
	pterm.Println("installing netmaker version ", latest)
	//get files
	baseURL := "https://raw.github.com/gravitl/netmaker/" + latest
	getFile(baseURL, "/compose/docker-compose.yml", "./docker-compose.yml")
	getFile(baseURL, "/scripts/netmaker.default.env", "./netmaker.default.env")
	getFile(baseURL, "/scripts/nm-certs.sh", "./nm-certs.sh")
	getFile(baseURL, "/docker/mosquitto.conf", "./mosquitto.conf")
	getFile(baseURL, "/docker/wait.sh", "./wait.sh")
	if pro {
		getFile(baseURL, "/compose/docker-compose.pro.yml", "./docker-compose.override.yml")
		getFile(baseURL, "/docker/Caddyfile-pro", "./Caddyfile")
	} else {
		getFile(baseURL, "/docker/Caddyfile", "./Caddyfile")
	}
	os.Chmod("wait.sh", 0700)
	os.Chmod("nm-certs.sh", 0700)
	netEnv, err := godotenv.Read("./netmaker.default.env")
	if err != nil {
		panic(err)
	}
	netEnv["NM_EMAIL"] = email
	netEnv["NM_DOMAIN"] = domain
	netEnv["UI_IMAGE_TAG"] = latest
	netEnv["SERVER_IMAGE_TAG"] = latest
	netEnv["MASTER_KEY"] = masterkey
	netEnv["MQ_USERNAME"] = mqUsername
	netEnv["MQ_PASSWORD"] = mqPassword
	netEnv["TURN_USERNAME"] = turnUsername
	netEnv["TURN_PASSWORD"] = turnPassword
	netEnv["INSTALL_TYPE"] = "ce"
	if pro {
		netEnv["INSTALL_TYPE"] = "pro"
		netEnv["METRICS_EXPORTER"] = "on"
		netEnv["PROMETHEUS"] = "on"
		netEnv["NETMAKER_TENENT_ID"] = tenantID
		netEnv["LICENSE_KEY"] = license
		netEnv["SERVER_IMAGE_NAME"] = latest + "-ee"
	}
	if err := godotenv.Write(netEnv, "./netmaker.env"); err != nil {
		panic(err)
	}
	os.Symlink("netmaker.env", ".env")
	//Fetch/Update certs
	pterm.Println("\nGetting certificates")
	//ensure docker daemon is running
	_, err = script.Exec("systemctl start docker").Stdout()
	if err != nil {
		panic(err)
	}
	//fix nm-cert.sh  remove -it from docker run -it --rm .....
	if _, err := script.File("./nm-certs.sh").Replace("-it", "").WriteFile("./certs.sh"); err != nil {
		panic(err)
	}
	os.Chmod("./certs.sh", 0700)
	if _, err := script.Exec("./certs.sh").Stdout(); err != nil {
		panic(err)
	}
	pterm.Println("\nStarting containers...")
	if _, err := script.Exec("docker-compose -f docker-compose.yml up -d --force-recreate").Stdout(); err != nil {
		panic(err)
	}
	testConnection()
}

func getFile(baseURL, remote, local string) {
	req, err := http.NewRequest(http.MethodGet, baseURL+remote, nil)
	if err != nil {
		panic(err)
	}
	if _, err := script.Do(req).WriteFile(local); err != nil {
		panic(err)
	}
}

func getLatestRelease() string {
	request, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/gravitl/netmaker/releases/latest", nil)
	if err != nil {
		panic(err)
	}
	client := http.Client{
		Timeout: time.Second * 10,
	}
	release := Release{}
	resp, err := client.Do(request)
	if err != nil {
		panic(err)
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		panic(err)
	}
	return release.Version
}

func testConnection() {
	pterm.Println("\nTesting Server setup for https://api." + domain + "/api/server/status")

	if _, err := script.Get("https://api." + domain + "/api/server/status").Stdout(); err != nil {
		pterm.Println("unable to connect to server, please investigate")
		pterm.Println("Exiting...")
		os.Exit(1)
	}
}
