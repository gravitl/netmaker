package ncutils

import (
	"context"
	"fmt"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"log"
	"os/exec"
	"strconv"
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
