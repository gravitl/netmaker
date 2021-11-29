package ncutils

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// RunCmd - runs a local command
func RunCmd(command string, printerr bool) (string, error) {
	args := strings.Fields(command)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Wait()
	//cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: "/C \"" + command + "\""}
	out, err := cmd.CombinedOutput()
	if err != nil && printerr {
		log.Println("error running command:", command)
		log.Println(strings.TrimSuffix(string(out), "\n"))
	}
	return string(out), err
}

// RunCmd - runs a local command
func RunCmdFormatted(command string, printerr bool) (string, error) {
	var comSpec = os.Getenv("COMSPEC")
	if comSpec == "" {
		comSpec = os.Getenv("SystemRoot") + "\\System32\\cmd.exe"
	}
	cmd := exec.Command(comSpec)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: "/C \"" + command + "\""}
	cmd.Wait()
	out, err := cmd.CombinedOutput()
	if err != nil && printerr {
		log.Println("error running command:", command)
		log.Println(strings.TrimSuffix(string(out), "\n"))
	}
	return string(out), err
}

// CreateUserSpaceConf - creates a user space WireGuard conf
func CreateUserSpaceConf(address string, privatekey string, listenPort string, mtu int32, perskeepalive int32, peers []wgtypes.PeerConfig) (string, error) {
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
PrivateKey = %s
MTU = %s
%s

%s

`,
		address+"/32",
		privatekey,
		strconv.Itoa(int(mtu)),
		listenPortString,
		peersString)
	return config, nil
}
