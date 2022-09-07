package daemon

import (
	"log"
	"os"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

// SetupFreebsdDaemon -- sets up daemon for freebsd
func SetupFreebsdDaemon() error {
	binarypath, err := os.Executable()
	if err != nil {
		return err
	}

	_, err = os.Stat("/etc/netclient/config")
	if os.IsNotExist(err) {
		os.MkdirAll("/etc/netclient/config", 0744)
	} else if err != nil {
		log.Println("couldnt find or create /etc/netclient")
		return err
	}
	//install binary
	if ncutils.FileExists(EXEC_DIR + "netclient") {
		logger.Log(0, "updating netclient binary in ", EXEC_DIR)
	}
	err = ncutils.Copy(binarypath, EXEC_DIR+"netclient")
	if err != nil {
		log.Println(err)
		return err
	}

	rcFile := `#!/bin/sh
#
# PROVIDE: netclient
# REQUIRE: LOGIN
# KEYWORD: shutdown

# Description:
#    This script runs netclient as a service as root on boot

# How to use:
#    Place this file in /usr/local/etc/rc.d/
#    Add netclient="YES" to /etc/rc.config.d/netclient
#    To pass args, add netclient_args="daemon" to /etc/rc.config.d/netclient

# Freebsd rc library
. /etc/rc.subr

# General Info
name="netclient"            # Safe name of program
program_name="netclient"   # Name of exec
title="netclient"          # Title to display in top/htop

# RC.config vars
load_rc_config $name      # Loading rc config vars
: ${netclient_enable="YES"}  # Default: enable netclient
: ${netclient_runAs="root"} # Default: Run Node-RED as root

# Freebsd Setup
rcvar=netclient_enable                   # Enables the rc.conf YES/NO flag
pidfile="/var/run/${program_name}.pid" # File that allows the system to keep track of node-red status

# Env Setup
#export HOME=$( getent passwd "$netclient_runAs" | cut -d: -f6 ) # Gets the home directory of the runAs user

# Command Setup
exec_path="/sbin/${program_name}" # Path to the netclient exec
output_file="/var/log/${program_name}.log" # Path to netclient logs

# Command
command="/usr/sbin/daemon"
command_args="-r -t ${title} -u ${netclient_runAs} -o ${output_file} -P ${pidfile} ${exec_path} ${netclient_args}"

# Loading Config
load_rc_config ${name}
run_rc_command "$1"
`

	rcConfig := `netclient="YES"
netclient_args="daemon"`

	rcbytes := []byte(rcFile)
	if !ncutils.FileExists("/etc/rc.d/netclient") {
		err := os.WriteFile("/etc/rc.d/netclient", rcbytes, 0744)
		if err != nil {
			return err
		}
		rcConfigbytes := []byte(rcConfig)
		if !ncutils.FileExists("/etc/rc.conf.d/netclient") {
			err := os.WriteFile("/etc/rc.conf.d/netclient", rcConfigbytes, 0644)
			if err != nil {
				return err
			}
			FreebsdDaemon("start")
			return nil
		}
	}
	return nil
}

// FreebsdDaemon - accepts args to service netclient and applies
func FreebsdDaemon(command string) {
	_, _ = ncutils.RunCmdFormatted("service netclient "+command, true)
}

// CleanupFreebsd - removes config files and netclient binary
func CleanupFreebsd() {
	ncutils.RunCmd("service netclient stop", false)
	RemoveFreebsdDaemon()
	if err := os.RemoveAll(ncutils.GetNetclientPath()); err != nil {
		logger.Log(1, "Removing netclient configs: ", err.Error())
	}
	if err := os.Remove(EXEC_DIR + "netclient"); err != nil {
		logger.Log(1, "Removing netclient binary: ", err.Error())
	}
}

// RemoveFreebsdDaemon - remove freebsd daemon
func RemoveFreebsdDaemon() {
	if ncutils.FileExists("/etc/rc.d/netclient") {
		err := os.Remove("/etc/rc.d/netclient")
		if err != nil {
			logger.Log(0, "Error removing /etc/rc.d/netclient. Please investigate.")
		}
	}
	if ncutils.FileExists("/etc/rc.conf.d/netclient") {
		err := os.Remove("/etc/rc.conf.d/netclient")
		if err != nil {
			logger.Log(0, "Error removing /etc/rc.conf.d/netclient. Please investigate.")
		}
	}
}
