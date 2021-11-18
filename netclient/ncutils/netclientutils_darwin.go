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

func RunCmdFormatted(command string, printerr bool) (string, error) {
	return "", nil
}

// CreateUserSpaceConf - creates a user space WireGuard conf
func CreateUserSpaceConf(address string, privatekey string, listenPort string, mtu int32, fwmark int32, perskeepalive int32, peers []wgtypes.PeerConfig) (string, error) {
	peersString, err := parsePeers(perskeepalive, peers)
	var listenPortString string
	var fwmarkString string
	if mtu <= 0 {
		mtu = 1280
	}
	if listenPort != "" {
		listenPortString += "ListenPort = " + listenPort
	}
	if fwmark != 0 {
		fwmarkString += "FWMark = " + strconv.Itoa(int(fwmark))
	}
	if err != nil {
		return "", err
	}
	config := fmt.Sprintf(`[Interface]
Address = %s
PrivateKey = %s
MTU = %s
%s
%s

%s

`,
		address+"/32",
		privatekey,
		strconv.Itoa(int(mtu)),
		listenPortString,
		fwmarkString,
		peersString)
	return config, nil
}
