package ncutils

import (
	"fmt"
	"os"
	"strconv"
)

// PIDFILE - path/name of pid file
const PIDFILE = "/var/run/netclient.pid"

// SavePID - saves the pid of running program to disk
func SavePID() error {
	pid := os.Getpid()
	if err := os.WriteFile(PIDFILE, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		return fmt.Errorf("could not write to pid file %w", err)
	}
	return nil
}

// ReadPID - reads a previously saved pid from disk
func ReadPID() (int, error) {
	bytes, err := os.ReadFile(PIDFILE)
	if err != nil {
		return 0, fmt.Errorf("could not read pid file %w", err)
	}
	pid, err := strconv.Atoi(string(bytes))
	if err != nil {
		return 0, fmt.Errorf("pid file contents invalid %w", err)
	}
	return pid, nil
}
