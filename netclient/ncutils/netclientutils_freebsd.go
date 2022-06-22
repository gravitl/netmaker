package ncutils

import (
	"context"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/gravitl/netmaker/logger"
)

// RunCmdFormatted - run a command formatted for freebsd
func RunCmdFormatted(command string, printerr bool) (string, error) {

	args := strings.Fields(command)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Start()
	cmd.Wait()
	out, err := cmd.CombinedOutput()
	if err != nil && printerr {
		logger.Log(0, "error running command: ", command)
		logger.Log(0, strings.TrimSuffix(string(out), "\n"))
	}
	return string(out), err
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
		logger.Log(0, "error running command:", command)
		logger.Log(0, strings.TrimSuffix(string(out), "\n"))
	}
	return string(out), err
}
