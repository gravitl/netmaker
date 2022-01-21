package ncutils

import (
	"context"
	"log"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// RunCmdFormatted - run a command formatted for freebsd
func RunCmdFormatted(command string, printerr bool) (string, error) {
	return "", nil
}

// GetEmbedded - if files required for freebsd, put here
func GetEmbedded() error {
	return nil
}

// Runs Commands for FreeBSD
func RunCmd(command string, printerr bool) (string, error) {
	args := strings.Fields(command)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	go func() {
		<-ctx.Done()
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}()
	out, err := cmd.CombinedOutput()
	if err != nil && printerr {
		log.Println("error running command:", command)
		log.Println(strings.TrimSuffix(string(out), "\n"))
	}
	return string(out), err
}

// CreateWireGuardConf - creates a WireGuard conf string
//func CreateWireGuardConf(node *models.Node, privatekey string, listenPort string, peers []wgtypes.PeerConfig) (string, error) {
//	peersString, err := parsePeers(node.PersistentKeepalive, peers)
//	var listenPortString string
//	if node.MTU <= 0 {
//		node.MTU = 1280
//	}
//	if listenPort != "" {
//		listenPortString += "ListenPort = " + listenPort
//	}
//	if err != nil {
//		return "", err
//	}
//	config := fmt.Sprintf(`[Interface]
//Address = %s
//PrivateKey = %s
//MTU = %s
//%s
//
//%s
//
//`,
//		node.Address+"/32",
//		privatekey,
//		strconv.Itoa(int(node.MTU)),
//		listenPortString,
//		peersString)
//	return config, nil
//}
