package ncutils

import (
	"fmt"
	"os"
	"strconv"
)

// PIDFILE - path/name of pid file
const PIDFILE = "/var/run/netclient.pid"
const WIN_PIDFILE = "C:\\Windows\\Temp\\netclient.pid"

// SavePID - saves the pid of running program to disk
func SavePID() error {
	pidfile := PIDFILE
	if IsWindows() {
		pidfile = WIN_PIDFILE
	}
	pid := os.Getpid()
	if err := os.WriteFile(pidfile, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		return fmt.Errorf("could not write to pid file %w", err)
	}
	return nil
}

// ReadPID - reads a previously saved pid from disk
func ReadPID() (int, error) {
	pidfile := PIDFILE
	if IsWindows() {
		pidfile = WIN_PIDFILE
	}
	bytes, err := os.ReadFile(pidfile)
	if err != nil {
		return 0, fmt.Errorf("could not read pid file %w", err)
	}
	pid, err := strconv.Atoi(string(bytes))
	if err != nil {
		return 0, fmt.Errorf("pid file contents invalid %w", err)
	}
	return pid, nil
}
