package ncutils

import (
	"fmt"
	"os"
	"strconv"
)

// PIDFILE - path/name of pid file
const PIDFILE = "/var/run/netclient.pid"

// WindowsPIDError - error returned from pid function on windows
type WindowsPIDError struct{}

// Error generates error for windows os
func (*WindowsPIDError) Error() string {
	return "pid tracking not supported on windows"
}

// SavePID - saves the pid of running program to disk
func SavePID() error {
	if IsWindows() {
		return nil
	}
	pid := os.Getpid()
	if err := os.WriteFile(PIDFILE, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		return fmt.Errorf("could not write to pid file %w", err)
	}
	return nil
}

// ReadPID - reads a previously saved pid from disk
func ReadPID() (int, error) {
	if IsWindows() {
		return 0, nil
	}
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
