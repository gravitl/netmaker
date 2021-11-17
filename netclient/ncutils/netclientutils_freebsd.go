package ncutils

import (
	"context"
	"log"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

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
