package ncutils

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// RunCmdFormatted - run a command formatted for freebsd
func RunCmdFormatted(command string, printerr bool) (string, error) {

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
		select {
		case <-ctx.Done():
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		case <-time.After(time.Second * 2):
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()
	cmd.Wait()
	out, err := cmd.CombinedOutput()
	if err != nil && printerr {
		log.Println("error running command:", command)
		log.Println(strings.TrimSuffix(string(out), "\n"))
	}
	return string(out), err
}
