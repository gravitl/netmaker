package ncutils

import (
	"log"
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
                log.Println("error running command:", command)
                log.Println(strings.TrimSuffix(string(out), "\n"))
        }
        return string(out), err
}

