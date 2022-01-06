package ncutils

import (
	"embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/gravitl/netmaker/models"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

//go:embed windowsdaemon/winsw.exe
var winswContent embed.FS

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

// GetEmbedded - Gets the Windows daemon creator
func GetEmbedded() error {
	data, err := winswContent.ReadFile("windowsdaemon/winsw.exe")
	if err != nil {
		return err
	}
	fileName := fmt.Sprintf("%swinsw.exe", GetNetclientPathSpecific())
	err = os.WriteFile(fileName, data, 0700)
	if err != nil {
		Log("could not mount winsw.exe")
		return err
	}
	return nil
}
