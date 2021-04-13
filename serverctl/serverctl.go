package serverctl

import (
        "fmt"
  "github.com/gravitl/netmaker/functions"
	"io"
	"net/http"
        "os"
        "os/exec"
)


func DownloadNetclient() error {

	// Get the data
	resp, err := http.Get("https://github.com/gravitl/netmaker/releases/download/latest/netclient")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create("/etc/netclient/netclient")
	if err != nil {
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
                fmt.Println("couldnt find or create /etc/netclient")
                return false, err
        }
	token, err := functions.CreateServerToken(network)
        if err != nil {
                return false, err
        }
        _, err = os.Stat("/etc/netclient/netclient")
	if os.IsNotExist(err) {
		err = DownloadNetclient()
		if err != nil {
			return false, err
		}
	}
        err = os.Chmod("/etc/netclient/netclient", 0755)
        if err != nil {
                return false, err
        }
	cmdoutput, err := exec.Command("/etc/netclient/netclient","-c","install","-t",token,"-name","netmaker").Output()
	if err != nil {
	        fmt.Println(string(cmdoutput))
                return false, err
        }
	fmt.Println("Server added to network " + network)
	return true, err
}


