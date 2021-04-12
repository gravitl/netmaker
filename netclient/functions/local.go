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


func FileExists(f string) bool {
    info, err := os.Stat(f)
    if os.IsNotExist(err) {
        return false
    }
    return !info.IsDir()
}

func ConfigureSystemD(network string) error {
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

	if !FileExists("/usr/local/bin/netclient") {
		os.Symlink("/etc/netclient/netclient","/usr/local/bin/netclient")
	/*
	_, err = copy(binarypath, "/usr/local/bin/netclient")
	if err != nil {
		log.Println(err)
		return err
	}
	*/
	}
	if !FileExists("/etc/netclient/netclient") {
        _, err = copy(binarypath, "/etc/netclient/netclient")
        if err != nil {
                log.Println(err)
                return err
        }
	}


	systemservice := `[Unit]
Description=Regularly checks for updates in peers and local config
Wants=netclient.timer

[Service]
Type=simple
ExecStart=/etc/netclient/netclient -c checkin -n %i

[Install]
WantedBy=multi-user.target
`

	systemtimer := `[Unit]
Description=Calls the Netmaker Mesh Client Service

`
systemtimer = systemtimer + "Requires=netclient@"+network+".service"

systemtimer = systemtimer +
`

[Timer]

`
systemtimer = systemtimer + "Unit=netclient@"+network+".service"

systemtimer = systemtimer +
`

OnCalendar=*:*:0/30

[Install]
WantedBy=timers.target
`


	servicebytes := []byte(systemservice)
	timerbytes := []byte(systemtimer)

	if !FileExists("/etc/systemd/system/netclient@.service") {
	err = ioutil.WriteFile("/etc/systemd/system/netclient@.service", servicebytes, 0644)
        if err != nil {
                log.Println(err)
                return err
        }
	}

        if !FileExists("/etc/systemd/system/netclient-"+network+".timer") {
        err = ioutil.WriteFile("/etc/systemd/system/netclient-"+network+".timer", timerbytes, 0644)
        if err != nil {
                log.Println(err)
                return err
        }
	}
        sysExec, err := exec.LookPath("systemctl")

        cmdSysEnableService := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "enable", "netclient@.service" },
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
	/*
        cmdSysStartService := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "start", "netclient@.service"},
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
	*/
        cmdSysDaemonReload := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "daemon-reload"},
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        cmdSysEnableTimer := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "enable", "netclient-"+network+".timer" },
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        cmdSysStartTimer := &exec.Cmd {
                Path: sysExec,
		Args: []string{ sysExec, "start", "netclient-"+network+".timer"},
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }

        err = cmdSysEnableService.Run()
        if  err  !=  nil {
                fmt.Println("Error enabling netclient@.service. Please investigate.")
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
                fmt.Println("Error starting netclient-"+network+".timer. Please investigate.")
                fmt.Println(err)
        }
	return nil
}

func isOnlyService(network string) (bool, error) {
	isonly := false
	files, err := filepath.Glob("/etc/netclient/netconfig-*")
	if err != nil {
		return isonly, err
	}
	count := len(files)
	if count  == 0 {
		isonly = true
	}
	return isonly, err

}

func RemoveSystemDServices(network string) error {
        sysExec, err := exec.LookPath("systemctl")


	fullremove, err := isOnlyService(network)
	if err != nil {
		fmt.Println(err)
	}

        cmdSysDisableService := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "disable", "netclient@.service"},
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
                Args: []string{ sysExec, "stop", "netclient-"+network+".timer" },
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        cmdSysDisableTimer := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "disable", "netclient-"+network+".timer"},
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }

        //err = cmdSysStopService.Run()
        if  err  !=  nil {
                fmt.Println("Error stopping netclient@.service. Please investigate.")
                fmt.Println(err)
        }
	if fullremove {
        err = cmdSysDisableService.Run()
        if  err  !=  nil {
                fmt.Println("Error disabling netclient@.service. Please investigate.")
                fmt.Println(err)
        }
	}
        err = cmdSysStopTimer.Run()
        if  err  !=  nil {
                fmt.Println("Error stopping netclient-"+network+".timer. Please investigate.")
                fmt.Println(err)
        }
        err = cmdSysDisableTimer.Run()
        if  err  !=  nil {
                fmt.Println("Error disabling netclient-"+network+".timer. Please investigate.")
                fmt.Println(err)
        }
	if fullremove {
	err = os.Remove("/etc/systemd/system/netclient@.service")
	}
	err = os.Remove("/etc/systemd/system/netclient-"+network+".timer")
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
