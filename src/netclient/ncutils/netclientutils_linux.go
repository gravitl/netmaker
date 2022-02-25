package ncutils

import (
	"fmt"
	"os/exec"
	"strings"
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


