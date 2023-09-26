package cmd

import (
	"net/http"
	"os"
	"runtime"

	"github.com/bitfield/script"
	"github.com/pterm/pterm"
)

func installNmctl() {
	pterm.Println("installing nmctl")
	arch := runtime.GOARCH
	baseURL := "https://github.com/gravitl/netmaker/releases/download/" + latest + "/nmctl-linux-" + arch
	getFile(baseURL, "", "/tmp/nmctl")
	os.Chmod("/tmp/nmctl", 0700)
	if _, err := script.Exec("/tmp/nmctl context set default --endpoint=https://api." + domain + " --master_key=" + masterkey).Stdout(); err != nil {
		panic(err)
	}
	if _, err := script.Exec("/tmp/nmctl context use default").Stdout(); err != nil {
		panic(err)
	}
	if _, err := script.Exec("/tmp/nmctl network list").Stdout(); err != nil {
		panic(err)
	}
}

func createNetwork() {
	request, err := http.NewRequest(http.MethodGet, "https://api."+domain+"/api/networks", nil)
	if err != nil {
		panic(err)
	}
	request.Header.Set("Authorization", "Bearer "+masterkey)
	resp, err := script.Do(request).String()
	if err != nil {
		panic(err)
	}
	if len(resp) > 5 {
		pterm.Println("networks exist, skipping creation of new network")
		return
	}
	pterm.Println("Creating netmaker network (10.101.0.0/16)")
	if _, err := script.Exec("/tmp/nmctl network create --name netmaker --ipv4_addr 10.101.0.0/16").String(); err != nil {
		panic(err)
	}
	pterm.Println("creating enrollmentkey")
	token, err = script.Exec("/tmp/nmctl enrollment_key create --uses 1 --networks netmaker --tags netmaker").JQ(" .token").String()
	if err != nil {
		panic(err)
	}
	pterm.Println("enrollment token:", token)
}
