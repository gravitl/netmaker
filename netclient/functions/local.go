package functions

import (
        //"github.com/davecgh/go-spew/spew"
        "fmt"
        "io/ioutil"
        "io"
        "log"
        "os"
        "os/exec"
)

func ConfigureSystemD() error {

	path, err := os.Getwd()
	if err != nil {
		log.Println(err)
		return err
	}

	binarypath := path  + "/meshclient"

	_, err = copy(binarypath, "/usr/local/bin/meshclient")
	if err != nil {
		log.Println(err)
		return err
	}


	systemservice := `[Unit]
Description=Regularly checks for updates in peers and local config
Wants=wirecat.timer

[Service]
Type=oneshot
ExecStart=/usr/local/bin/meshclient -c checkin

[Install]
WantedBy=multi-user.target
`

	systemtimer := `[Unit]
Description=Calls the WireCat Mesh Client Service
Requires=wirecat.service

[Timer]
Unit=wirecat.service
OnCalendar=*:*:0/30

[Install]
WantedBy=timers.target
`

	servicebytes := []byte(systemservice)
	timerbytes := []byte(systemtimer)

	err = ioutil.WriteFile("/etc/systemd/system/wirecat.service", servicebytes, 0644)
        if err != nil {
                log.Println(err)
                return err
        }

        err = ioutil.WriteFile("/etc/systemd/system/wirecat.timer", timerbytes, 0644)
        if err != nil {
                log.Println(err)
                return err
        }

        sysExec, err := exec.LookPath("systemctl")

        cmdSysEnableService := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "enable", "wirecat.service" },
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        cmdSysStartService := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "start", "wirecat.service"},
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
                Args: []string{ sysExec, "enable", "wirecat.timer" },
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        cmdSysStartTimer := &exec.Cmd {
                Path: sysExec,
		Args: []string{ sysExec, "start", "wirecat.timer"},
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }

        err = cmdSysEnableService.Run()
        if  err  !=  nil {
                fmt.Println("Error enabling wirecat.service. Please investigate.")
                fmt.Println(err)
        }
        err = cmdSysStartService.Run()
        if  err  !=  nil {
                fmt.Println("Error starting wirecat.service. Please investigate.")
                fmt.Println(err)
        }
        err = cmdSysDaemonReload.Run()
        if  err  !=  nil {
                fmt.Println("Error reloading system daemons. Please investigate.")
                fmt.Println(err)
        }
        err = cmdSysEnableTimer.Run()
        if  err  !=  nil {
                fmt.Println("Error enabling wirecat.timer. Please investigate.")
                fmt.Println(err)
        }
        err = cmdSysStartTimer.Run()
        if  err  !=  nil {
                fmt.Println("Error starting wirecat.timer. Please investigate.")
                fmt.Println(err)
        }
	return nil
}

func RemoveSystemDServices() error {
        sysExec, err := exec.LookPath("systemctl")

        cmdSysStopService := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "stop", "wirecat.service" },
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        cmdSysDisableService := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "disable", "wirecat.service"},
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        cmdSysDaemonReload := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "daemon-reload"},
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        cmdSysStopTimer := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "stop", "wirecat.timer" },
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        cmdSysDisableTimer := &exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "disable", "wirecat.timer"},
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }

        err = cmdSysStopService.Run()
        if  err  !=  nil {
                fmt.Println("Error stopping wirecat.service. Please investigate.")
                fmt.Println(err)
        }
        err = cmdSysDisableService.Run()
        if  err  !=  nil {
                fmt.Println("Error disabling wirecat.service. Please investigate.")
                fmt.Println(err)
        }
        err = cmdSysStopTimer.Run()
        if  err  !=  nil {
                fmt.Println("Error stopping wirecat.timer. Please investigate.")
                fmt.Println(err)
        }
        err = cmdSysDisableTimer.Run()
        if  err  !=  nil {
                fmt.Println("Error disabling wirecat.timer. Please investigate.")
                fmt.Println(err)
        }

	err = os.Remove("/etc/systemd/system/wirecat.service")
	err = os.Remove("/etc/systemd/system/wirecat.timer")
        //err = os.Remove("/usr/local/bin/meshclient")
	if err != nil {
                fmt.Println("Error removing file. Please investigate.")
                fmt.Println(err)
	}
        err = cmdSysDaemonReload.Run()
        if  err  !=  nil {
                fmt.Println("Error reloading system daemons. Please investigate.")
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
