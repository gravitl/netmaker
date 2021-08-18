package local

import (
	//"github.com/davecgh/go-spew/spew"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gravitl/netmaker/netclient/config"
)

func SetIPForwarding() error {
	os := runtime.GOOS
	var err error
	switch os {
	case "linux":
		err = SetIPForwardingLinux()
	default:
		err = errors.New("This OS is not supported")
	}
	return err
}

func SetIPForwardingLinux() error {
	out, err := RunCmd("sysctl net.ipv4.ip_forward")
	if err != nil {
		log.Println(err)
		log.Println("WARNING: Error encountered setting ip forwarding. This can break functionality.")
		return err
	} else {
		s := strings.Fields(string(out))
		if s[2] != "1" {
			_, err = RunCmd("sysctl -w net.ipv4.ip_forward=1")
			if err != nil {
				log.Println(err)
				log.Println("WARNING: Error encountered setting ip forwarding. You may want to investigate this.")
				return err
			}
		}
	}
	return nil
}

func RunCmd(command string) (string, error) {
	args := strings.Fields(command)
	out, err := exec.Command(args[0], args[1:]...).Output()
	return string(out), err
}

func RunCmds(commands []string) error {
	var err error
	for _, command := range commands {
		args := strings.Fields(command)
		out, err := exec.Command(args[0], args[1:]...).Output()
		if string(out) != "" {
			log.Println(string(out))
		}
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
	binarypath := dir + "/netclient"

	_, err = os.Stat("/etc/netclient")
	if os.IsNotExist(err) {
		os.Mkdir("/etc/netclient", 744)
	} else if err != nil {
		log.Println("couldnt find or create /etc/netclient")
		return err
	}

	if !FileExists("/usr/local/bin/netclient") {
		os.Symlink("/etc/netclient/netclient", "/usr/local/bin/netclient")
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
Description=Network Check
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
	systemtimer = systemtimer + "Requires=netclient@" + network + ".service"

	systemtimer = systemtimer +
		`

[Timer]

`
	systemtimer = systemtimer + "Unit=netclient@" + network + ".service"

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

	if !FileExists("/etc/systemd/system/netclient-" + network + ".timer") {
		err = ioutil.WriteFile("/etc/systemd/system/netclient-"+network+".timer", timerbytes, 0644)
		if err != nil {
			log.Println(err)
			return err
		}
	}

	_, err = RunCmd("systemctl enable netclient@.service")
	if err != nil {
		log.Println("Error enabling netclient@.service. Please investigate.")
		log.Println(err)
	}
	_, err = RunCmd("systemctl daemon-reload")
	if err != nil {
		log.Println("Error reloading system daemons. Please investigate.")
		log.Println(err)
	}
	_, err = RunCmd("systemctl enable netclient-" + network + ".timer")
	if err != nil {
		log.Println("Error enabling netclient.timer. Please investigate.")
		log.Println(err)
	}
	_, err = RunCmd("systemctl start netclient-" + network + ".timer")
	if err != nil {
		log.Println("Error starting netclient-" + network + ".timer. Please investigate.")
		log.Println(err)
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
	if count == 0 {
		isonly = true
	}
	return isonly, err

}

func RemoveSystemDServices(network string) error {
	//sysExec, err := exec.LookPath("systemctl")

	fullremove, err := isOnlyService(network)
	if err != nil {
		log.Println(err)
	}

	if fullremove {
		_, err = RunCmd("systemctl disable netclient@.service")
		if err != nil {
			log.Println("Error disabling netclient@.service. Please investigate.")
			log.Println(err)
		}
	}
	_, err = RunCmd("systemctl daemon-reload")
	if err != nil {
		log.Println("Error stopping netclient-" + network + ".timer. Please investigate.")
		log.Println(err)
	}
	_, err = RunCmd("systemctl disable netclient-" + network + ".timer")
	if err != nil {
		log.Println("Error disabling netclient-" + network + ".timer. Please investigate.")
		log.Println(err)
	}
	if fullremove {
		if FileExists("/etc/systemd/system/netclient@.service") {
			err = os.Remove("/etc/systemd/system/netclient@.service")
		}
	}
	if FileExists("/etc/systemd/system/netclient-" + network + ".timer") {
		err = os.Remove("/etc/systemd/system/netclient-" + network + ".timer")
	}
	if err != nil {
		log.Println("Error removing file. Please investigate.")
		log.Println(err)
	}
	_, err = RunCmd("systemctl daemon-reload")
	if err != nil {
		log.Println("Error reloading system daemons. Please investigate.")
		log.Println(err)
	}
	_, err = RunCmd("systemctl reset-failed")
	if err != nil {
		log.Println("Error reseting failed system services. Please investigate.")
		log.Println(err)
	}
	return err
}

func WipeLocal(network string) error {
	cfg, err := config.ReadConfig(network)
	if err != nil {
		return err
	}
	nodecfg := cfg.Node
	ifacename := nodecfg.Interface

	//home, err := homedir.Dir()
	home := "/etc/netclient"
	if FileExists(home + "/netconfig-" + network) {
		_ = os.Remove(home + "/netconfig-" + network)
	}
	if FileExists(home + "/nettoken-" + network) {
		_ = os.Remove(home + "/nettoken-" + network)
	}
	if FileExists(home + "/wgkey-" + network) {
		_ = os.Remove(home + "/wgkey-" + network)
	}

	ipExec, err := exec.LookPath("ip")
	if err != nil {
		return err
	}
	if ifacename != "" {
		out, err := RunCmd(ipExec + " link del " + ifacename)
		if err != nil {
			log.Println(out, err)
		}
		if nodecfg.PostDown != "" {
			runcmds := strings.Split(nodecfg.PostDown, "; ")
			err = RunCmds(runcmds)
			if err != nil {
				log.Println("Error encountered running PostDown: " + err.Error())
			}
		}
	}
	return err

}

func HasNetwork(network string) bool {

	return FileExists("/etc/systemd/system/netclient-"+network+".timer") ||
		FileExists("/etc/netclient/netconfig-"+network)

}

func copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, errors.New(src + " is not a regular file")
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
