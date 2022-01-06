package ncutils

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gravitl/netmaker/models"
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

// RunCmdFormatted - run a command formatted for MacOS
func RunCmdFormatted(command string, printerr bool) (string, error) {
	return "", nil
}

// GetEmbedded - if files required for MacOS, put here
func GetEmbedded() error {
	return nil
}

// CreateWireGuardConf - creates a WireGuard conf string
func CreateWireGuardConf(node *models.Node, privatekey string, listenPort string, peers []wgtypes.PeerConfig) (string, error) {
	peersString, err := parsePeers(node.PersistentKeepalive, peers)
	var listenPortString string
	if node.MTU <= 0 {
		node.MTU = 1280
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
		node.Address+"/32",
		privatekey,
		strconv.Itoa(int(node.MTU)),
		listenPortString,
		peersString)
	return config, nil
}
