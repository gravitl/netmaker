package local

import (
        //"github.com/davecgh/go-spew/spew"
        "github.com/gravitl/netmaker/netclient/config"
	"fmt"
        "io/ioutil"
	"path/filepath"
        "io"
	"strings"
        "log"
        "os"
        "os/exec"
)

func RunCmds(commands []string) error {
        var err error
        for _, command := range commands {
                fmt.Println("Running command: " + command)
                args := strings.Fields(command)
                out, err := exec.Command(args[0], args[1:]...).Output()
                fmt.Println(string(out))
                if err != nil {
                        return err
                }
        }
        return err
}

func FileExists(f string) bool {
    info, err := os.Stat(f)
    if os.IsNotExist(err) {
        return false
    }
    return !info.IsDir()
}

func SetDNS(nameserver string) error {
	bytes, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		return err
	}
	resolvstring := string(bytes)
	// //check whether s contains substring text
	hasdns := strings.Contains(resolvstring, nameserver)
	if hasdns {
		return nil
	}
	resolv, err := os.OpenFile("/etc/resolv.conf",os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer resolv.Close()
	_, err = resolv.WriteString("nameserver " + nameserver + "\n")

	return err
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
Description=network check for remote peers and local config
Wants=netclient.timer

[Service]
Type=simple
ExecStart=/etc/netclient/netclient checkin -n %i

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
        //sysExec, err := exec.LookPath("systemctl")

        cmdSysEnableService := exec.Command("systemctl", "enable", "netclient@.service")/*&exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "enable", "netclient@.service" },
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }*/
        cmdSysDaemonReload := exec.Command("systemctl", "daemon-reload")/*&exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "daemon-reload"},
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }*/
        cmdSysEnableTimer := exec.Command("systemctl", "enable", "netclient-"+network+".timer")/*&exec.Cmd {
                Path: sysExec,
                Args: []string{ sysExec, "enable", "netclient-"+network+".timer" },
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }*/
        cmdSysStartTimer := exec.Command("systemctl", "start", "netclient-"+network+".timer")/*&exec.Cmd {
                Path: sysExec,
		Args: []string{ sysExec, "start", "netclient-"+network+".timer"},
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }*/

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
        //sysExec, err := exec.LookPath("systemctl")


	fullremove, err := isOnlyService(network)
	if err != nil {
		fmt.Println(err)
	}

	cmdSysDisableService := exec.Command("systemctl","disable","netclient@.service")
        cmdSysDaemonReload := exec.Command("systemctl","daemon-reload")
        cmdSysResetFailed := exec.Command("systemctl","reset-failed")
        cmdSysStopTimer := exec.Command("systemctl", "stop", "netclient-"+network+".timer")
        cmdSysDisableTimer :=  exec.Command("systemctl", "disable", "netclient-"+network+".timer")
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

func WipeLocal(network string) error{
        cfg, err := config.ReadConfig(network)
        if err != nil {
                return err
        }
        nodecfg := cfg.Node
        ifacename := nodecfg.Interface

        //home, err := homedir.Dir()
        home := "/etc/netclient"
        _ = os.Remove(home + "/netconfig-" + network)
        _ = os.Remove(home + "/nettoken-" + network)
        _ = os.Remove(home + "/wgkey-" + network)

        ipExec, err := exec.LookPath("ip")

        if ifacename != "" {
        cmdIPLinkDel := &exec.Cmd {
                Path: ipExec,
                Args: []string{ ipExec, "link", "del", ifacename },
                Stdout: os.Stdout,
                Stderr: os.Stdout,
        }
        err = cmdIPLinkDel.Run()
        if  err  !=  nil {
                fmt.Println(err)
        }
        if nodecfg.PostDown != "" {
                runcmds := strings.Split(nodecfg.PostDown, "; ")
                err = RunCmds(runcmds)
                if err != nil {
                        fmt.Println("Error encountered running PostDown: " + err.Error())
                }
        }
        }
        return err

}

func HasNetwork(network string) bool{

return  FileExists("/etc/systemd/system/netclient-"+network+".timer") ||
	FileExists("/etc/netclient/netconfig-"+network)

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
