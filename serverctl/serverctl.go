package serverctl

import (
        "fmt"
        "io/ioutil"
        "log"
        "os"
        "os/exec"
)


func fileExists(f string) bool {
    info, err := os.Stat(f)
    if os.IsNotExist(err) {
        return false
    }
    notisdir := !info.IsDir()
    return notisdir
}


func installScript() error {


	installScript := `#!/bin/sh
set -e

[ -z "$SERVER_URL" ] && echo "Need to set SERVER_URL" && exit 1;
[ -z "$NET_NAME" ] && echo "Need to set NET_NAME" && exit 1;
[ -z "$KEY" ] && KEY=nokey;



wget -O netclient https://github.com/gravitl/netmaker/releases/download/develop/netclient
chmod +x netclient
sudo ./netclient -c install -s $SERVER_URL -g $NET_NAME -k $KEY
rm -f netclient
`

        installbytes := []byte(installScript)

	err := ioutil.WriteFile("/etc/netclient/netclient-install.sh", installbytes, 0755)
        if err != nil {
                log.Println(err)
                return err
        }
	return err
}


func NetworkAdd(network string) error {
	_, err := os.Stat("/etc/netclient")
        if os.IsNotExist(err) {
                os.Mkdir("/etc/netclient", 744)
        } else if err != nil {
                fmt.Println("couldnt find or create /etc/netclient")
                return err
        }
        if !fileExists("/etc/netclient/netclient-install.sh") {
        err = installScript()
        if err != nil {
                log.Println(err)
                return err
        }
	}

	cmdoutput, err := exec.Command("/bin/sh", "/etc/netclient/netclient-install.sh").Output()
	if err != nil {
		fmt.Printf("Error installing netclient: %s", err)
	}
	fmt.Println(cmdoutput)
	return err
}


