package functions

import (
        //"github.com/davecgh/go-spew/spew"
        "fmt"
        "io/ioutil"
	"path/filepath"
        "io"
        "log"
        "os"
        "os/exec"
)

func ConfigureSystemD() error {
	/*
	path, err := os.Getwd()
	if err != nil {
		log.Println(err)
		return err
	}
	*/
	//binarypath := path  + "/netclient"
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
            return err
	}
	binarypath := dir  + "/netclient"

	fmt.Println("Installing Binary from Path: " + binarypath)

	_, err = os.Stat("/etc/netclient")
        if os.IsNotExist(err) {
                os.Mkdir("/etc/netclient", 744)
        } else if err != nil {
                fmt.Println("couldnt find or create /etc/netclient")
                return err
        }

	_, err = copy(binarypath, "/usr/local/bin/netclient")
	if err != nil {
		log.Println(err)
		return err
	}
        _, err = copy(binarypath, "/etc/netclient/netclient")
        if err != nil {
                log.Println(err)
                return err
        }



	systemservice := `[Unit]
Description=Regularly checks for updates in peers and local config
Wants=netclient.timer

[Service]
Type=oneshot
ExecStart=/etc/netclient/netclient -c checkin

[Install]
WantedBy=multi-user.target
`

	systemtimer := `[Unit]
Description=Calls the Netmaker Mesh Client Service
Requires=netclient.service

[Timer]
Unit=netclient.service
OnCalendar=*:*:0/30

[Install]
WantedBy=timers.target
`

	servicebytes := []byte(systemservice)
	timerbytes := []byte(systemtimer)

	err = ioutil.WriteFile("/etc/systemd/system/netclient.service", servicebytes, 0644)
        if err != nil {
                log.Println(err)
                return err
        }

        err = ioutil.WriteFile("/etc/systemd/system/netclient.timer", timerbytes, 0644)
        if err != nil {
                log.Println(err)
                return err
        }

        sysExec, err := exec.LookPath("systemctl")

        cmdSysEnableService := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "enable", "netclient.service" },
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        cmdSysStartService := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "start", "netclient.service"},
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        cmdSysDaemonReload := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "daemon-reload"},
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        cmdSysEnableTimer := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "enable", "netclient.timer" },
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        cmdSysStartTimer := &exec.Cmd {
                Path: sysExec,
		Args: []string{ sysExec, "start", "netclient.timer"},
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }

        err = cmdSysEnableService.Run()
        if  err  !=  nil {
                fmt.Println("Error enabling netclient.service. Please investigate.")
                fmt.Println(err)
        }
        err = cmdSysStartService.Run()
        if  err  !=  nil {
                fmt.Println("Error starting netclient.service. Please investigate.")
                fmt.Println(err)
        }
        err = cmdSysDaemonReload.Run()
        if  err  !=  nil {
                fmt.Println("Error reloading system daemons. Please investigate.")
                fmt.Println(err)
        }
        err = cmdSysEnableTimer.Run()
        if  err  !=  nil {
                fmt.Println("Error enabling netclient.timer. Please investigate.")
                fmt.Println(err)
        }
        err = cmdSysStartTimer.Run()
        if  err  !=  nil {
                fmt.Println("Error starting netclient.timer. Please investigate.")
                fmt.Println(err)
        }
	return nil
}

func RemoveSystemDServices() error {
        sysExec, err := exec.LookPath("systemctl")

        cmdSysStopService := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "stop", "netclient.service" },
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        cmdSysDisableService := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "disable", "netclient.service"},
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        cmdSysDaemonReload := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "daemon-reload"},
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        cmdSysResetFailed := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "reset-failed"},
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        cmdSysStopTimer := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "stop", "netclient.timer" },
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        cmdSysDisableTimer := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "disable", "netclient.timer"},
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }

        err = cmdSysStopService.Run()
        if  err  !=  nil {
                fmt.Println("Error stopping netclient.service. Please investigate.")
                fmt.Println(err)
        }
        err = cmdSysDisableService.Run()
        if  err  !=  nil {
                fmt.Println("Error disabling netclient.service. Please investigate.")
                fmt.Println(err)
        }
        err = cmdSysStopTimer.Run()
        if  err  !=  nil {
                fmt.Println("Error stopping netclient.timer. Please investigate.")
                fmt.Println(err)
        }
        err = cmdSysDisableTimer.Run()
        if  err  !=  nil {
                fmt.Println("Error disabling netclient.timer. Please investigate.")
                fmt.Println(err)
        }

	err = os.Remove("/etc/systemd/system/netclient.service")
	err = os.Remove("/etc/systemd/system/netclient.timer")
	if err != nil {
                fmt.Println("Error removing file. Please investigate.")
                fmt.Println(err)
	}
        err = cmdSysDaemonReload.Run()
        if  err  !=  nil {
                fmt.Println("Error reloading system daemons. Please investigate.")
                fmt.Println(err)
        }
        err = cmdSysResetFailed.Run()
        if  err  !=  nil {
                fmt.Println("Error reseting failed system services. Please investigate.")
                fmt.Println(err)
        }
	return err

}

func copy(src, dst string) (int64, error) {
        sourceFileStat, err := os.Stat(src)
        if err != nil {
                return 0, err
        }

        if !sourceFileStat.Mode().IsRegular() {
                return 0, fmt.Errorf("%s is not a regular file", src)
        }

        source, err := os.Open(src)
        if err != nil {
                return 0, err
        }
        defer source.Close()

        destination, err := os.Create(dst)
        if err != nil {
                return 0, err
        }
        defer destination.Close()
        nBytes, err := io.Copy(destination, source)
	err = os.Chmod(dst, 0755)
	if err != nil {
		log.Println(err)
	}
        return nBytes, err
}
