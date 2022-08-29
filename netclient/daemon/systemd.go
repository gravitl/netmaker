package daemon

import (
	//"github.com/davecgh/go-spew/spew"

	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

const EXEC_DIR = "/sbin/"

// SetupSystemDDaemon - sets system daemon for supported machines
func SetupSystemDDaemon() error {

	if ncutils.IsWindows() {
		return nil
	}
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
		logger.Log(0, "updating netclient binary in", EXEC_DIR)
	}
	err = ncutils.Copy(binarypath, EXEC_DIR+"netclient")
	if err != nil {
		log.Println(err)
		return err
	}

	systemservice := `[Unit]
Description=Netclient Daemon
Documentation=https://docs.netmaker.org https://k8s.netmaker.org
After=network-online.target
Wants=network-online.target systemd-networkd-wait-online.service

[Service]
User=root
Type=simple
ExecStart=/sbin/netclient daemon
Restart=on-failure
RestartSec=15s

[Install]
WantedBy=multi-user.target
`

	servicebytes := []byte(systemservice)

	if !ncutils.FileExists("/etc/systemd/system/netclient.service") {
		err = os.WriteFile("/etc/systemd/system/netclient.service", servicebytes, 0644)
		if err != nil {
			log.Println(err)
			return err
		}
	}
	_, _ = ncutils.RunCmd("systemctl enable netclient.service", true)
	_, _ = ncutils.RunCmd("systemctl daemon-reload", true)
	_, _ = ncutils.RunCmd("systemctl start netclient.service", true)
	return nil
}

// RestartSystemD - restarts systemd service
func RestartSystemD() {
	logger.Log(1, "restarting netclient.service")
	time.Sleep(time.Second)
	_, _ = ncutils.RunCmd("systemctl restart netclient.service", true)
}

// CleanupLinux - cleans up neclient configs
func CleanupLinux() {
	if _, err := ncutils.RunCmd("systemctl stop netclient", false); err != nil {
		logger.Log(0, "failed to stop netclient service", err.Error())
	}
	RemoveSystemDServices()
	if err := os.RemoveAll(ncutils.GetNetclientPath()); err != nil {
		logger.Log(1, "Removing netclient configs: ", err.Error())
	}
	if err := os.Remove(EXEC_DIR + "netclient"); err != nil {
		logger.Log(1, "Removing netclient binary: ", err.Error())
	}
}

// StopSystemD - tells system to stop systemd
func StopSystemD() {
	ncutils.RunCmd("systemctl stop netclient.service", false)
}

// RemoveSystemDServices - removes the systemd services on a machine
func RemoveSystemDServices() error {
	//sysExec, err := exec.LookPath("systemctl")
	var err error
	if !ncutils.IsWindows() && isOnlyService() {
		if err != nil {
			log.Println(err)
		}
		ncutils.RunCmd("systemctl disable netclient.service", false)
		ncutils.RunCmd("systemctl disable netclient.timer", false)
		if ncutils.FileExists("/etc/systemd/system/netclient.service") {
			err = os.Remove("/etc/systemd/system/netclient.service")
			if err != nil {
				logger.Log(0, "Error removing /etc/systemd/system/netclient.service. Please investigate.")
			}
		}
		if ncutils.FileExists("/etc/systemd/system/netclient.timer") {
			err = os.Remove("/etc/systemd/system/netclient.timer")
			if err != nil {
				logger.Log(0, "Error removing /etc/systemd/system/netclient.timer. Please investigate.")
			}
		}
		ncutils.RunCmd("systemctl daemon-reload", false)
		ncutils.RunCmd("systemctl reset-failed", false)
		logger.Log(0, "removed systemd remnants if any existed")
	}
	return nil
}

func isOnlyService() bool {
	files, err := filepath.Glob("/etc/netclient/config/netconfig-*")
	if err != nil {
		return false
	}
	return len(files) == 0
}
