package ncutils

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// RunCmd - runs a local command
func RunCmd(command string, printerr bool) (string, error) {
	args := strings.Fields(command)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Wait()
	out, err := cmd.CombinedOutput()
	if err != nil && printerr {
		log.Println("error running command:", command)
		log.Println(strings.TrimSuffix(string(out), "\n"))
	}
	return string(out), err
}

// RunCmdFormatted - does nothing for linux
func RunCmdFormatted(command string, printerr bool) (string, error) {
	return "", nil
}

// GetEmbedded - if files required for linux, put here
func GetEmbedded() error {
	return nil
}

// CreateUserSpaceConf - creates a user space WireGuard conf
func CreateUserSpaceConf(address string, privatekey string, listenPort string, mtu int32, dns string, perskeepalive int32, peers []wgtypes.PeerConfig) (string, error) {
	log.Println("---------- dns passed to create conf ", dns)
	peersString, err := parsePeers(perskeepalive, peers)
	var listenPortString string
	if mtu <= 0 {
		mtu = 1280
	}
	if listenPort != "" {
		listenPortString += "ListenPort = " + listenPort
	}
	if err != nil {
		return "", err
	}
	config := fmt.Sprintf(`[Interface]
Address = %s
DNS = %s
PrivateKey = %s
MTU = %s
%s

%s

`,
		address+"/32",
		dns,
		privatekey,
		strconv.Itoa(int(mtu)),
		listenPortString,
		peersString)
	return config, nil
}
