package ncutils

import (
	"fmt"
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
		Log(fmt.Sprintf("error running command: %s", command))
		Log(strings.TrimSuffix(string(out), "\n"))
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

// CreateWireGuardConf - creates a user space WireGuard conf
func CreateWireGuardConf(node *models.Node, privatekey string, listenPort string, peers []wgtypes.PeerConfig) (string, error) {
	peersString, err := parsePeers(node.PersistentKeepalive, peers)
	var listenPortString, postDownString, postUpString string
	if node.MTU <= 0 {
		node.MTU = 1280
	}
	if node.PostDown != "" {
		postDownString = fmt.Sprintf("PostDown = %s", node.PostDown)
	}
	if node.PostUp != "" {
		postUpString = fmt.Sprintf("PostUp = %s", node.PostUp)
	}

	if listenPort != "" {
		listenPortString = fmt.Sprintf("ListenPort = %s", listenPort)
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

%s

`,
		node.Address+"/32",
		privatekey,
		strconv.Itoa(int(node.MTU)),
		postDownString,
		postUpString,
		listenPortString,
		peersString)
	return config, nil
}
