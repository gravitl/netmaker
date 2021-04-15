package serverctl

import (
        "fmt"
  "github.com/gravitl/netmaker/functions"
	"io"
	"errors"
	"net/http"
        "os"
        "os/exec"
)


func DownloadNetclient() error {

	// Get the data
	resp, err := http.Get("https://github.com/gravitl/netmaker/releases/download/latest/netclient")
	if err != nil {
                fmt.Println("could not download netclient")
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create("/etc/netclient/netclient")
	if err != nil {
                fmt.Println("could not create /etc/netclient")
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}
func RemoveNetwork(network string) (bool, error) {
	_, err := os.Stat("/etc/netclient/netclient")
        if err != nil {
                fmt.Println("could not find /etc/netclient")
		return false, err
	}
        cmdoutput, err := exec.Command("/etc/netclient/netclient","-c","remove","-n",network).Output()
        if err != nil {
                fmt.Println(string(cmdoutput))
                return false, err
        }
        fmt.Println("Server removed from network " + network)
        return true, err

}

func AddNetwork(network string) (bool, error) {
	_, err := os.Stat("/etc/netclient")
        if os.IsNotExist(err) {
                os.Mkdir("/etc/netclient", 744)
        } else if err != nil {
                fmt.Println("could not find or create /etc/netclient")
                return false, err
        }
	fmt.Println("Directory is ready.")
	token, err := functions.CreateServerToken(network)
        if err != nil {
                fmt.Println("could not create server token for " + network)
		return false, err
        }
	fmt.Println("Token is ready.")
        _, err = os.Stat("/etc/netclient/netclient")
	if os.IsNotExist(err) {
		err = DownloadNetclient()
                fmt.Println("could not download netclient")
		if err != nil {
			return false, err
		}
	}
        err = os.Chmod("/etc/netclient/netclient", 0755)
        if err != nil {
                fmt.Println("could not change netclient directory permissions")
                return false, err
        }
	fmt.Println("Client is ready. Running install.")
	out, err := exec.Command("/etc/netclient/netclient","-c","install","-t",token,"-name","netmaker").Output()
        fmt.Println(string(out))
	if err != nil {
                return false, errors.New(string(out) + err.Error())
        }
	fmt.Println("Server added to network " + network)
	return true, err
}

