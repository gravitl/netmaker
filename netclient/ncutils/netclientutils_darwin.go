package ncutils

import (
	"os/exec"
	"strings"

	"github.com/gravitl/netmaker/logger"
)

// WHITESPACE_PLACEHOLDER - used with RunCMD - if a path has whitespace, use this to avoid running path as 2 args in RunCMD
const WHITESPACE_PLACEHOLDER = "+-+-+-+"

// RunCmd - runs a local command
func RunCmd(command string, printerr bool) (string, error) {

	args := strings.Fields(command)
	// return whitespace after split
	for i, arg := range args {
		args[i] = strings.Replace(arg, WHITESPACE_PLACEHOLDER, " ", -1)
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Wait()
	out, err := cmd.CombinedOutput()
	if err != nil && printerr {
		logger.Log(0, "error running command:", strings.Join(args, " "))
		logger.Log(0, strings.TrimSuffix(string(out), "\n"))
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
