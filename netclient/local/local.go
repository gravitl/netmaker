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
	"github.com/gravitl/netmaker/netclient/netclientutils"
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
	out, err := RunCmd("sysctl net.ipv4.ip_forward", true)
	if err != nil {
		log.Println("WARNING: Error encountered setting ip forwarding. This can break functionality.")
		return err
	} else {
		s := strings.Fields(string(out))
		if s[2] != "1" {
			_, err = RunCmd("sysctl -w net.ipv4.ip_forward=1", true)
			if err != nil {
				log.Println("WARNING: Error encountered setting ip forwarding. You may want to investigate this.")
				return err
			}
		}
	}
	return nil
}

func RunCmd(command string, printerr bool) (string, error) {
	args := strings.Fields(command)
	out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil && printerr {
		log.Println("error running command:",command)
		log.Println(string(out))
	}
	return string(out), err
}

func RunCmds(commands []string, printerr bool) error {
	var err error
	for _, command := range commands {
		args := strings.Fields(command)
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		if err != nil && printerr {
			log.Println("error running command:",command)
			log.Println(string(out))
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
	if netclientutils.IsWindows() {
		return nil
	}
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

	_, _ = RunCmd("systemctl enable netclient@.service", true)
	_, _ = RunCmd("systemctl daemon-reload", true)
	_, _ = RunCmd("systemctl enable netclient-" + network + ".timer", true)
	_, _ = RunCmd("systemctl start netclient-" + network + ".timer", true)
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
	if !netclientutils.IsWindows() {
		fullremove, err := isOnlyService(network)
		if err != nil {
			log.Println(err)
		}

		if fullremove {
			_, err = RunCmd("systemctl disable netclient@.service", true)
		}
		_, _ = RunCmd("systemctl daemon-reload", true)

		if FileExists("/etc/systemd/system/netclient-" + network + ".timer") {
			_, _ = RunCmd("systemctl disable netclient-" + network + ".timer", true)
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
		_, _ = RunCmd("systemctl daemon-reload", true)
		_, _ = RunCmd("systemctl reset-failed", true)
	}
	return nil
}

func WipeLocal(network string) error {
	cfg, err := config.ReadConfig(network)
	if err != nil {
		return err
	}
	nodecfg := cfg.Node
	ifacename := nodecfg.Interface

	home := netclientutils.GetNetclientPathSpecific()
	if FileExists(home + "netconfig-" + network) {
		_ = os.Remove(home + "netconfig-" + network)
	}
	if FileExists(home + "nettoken-" + network) {
		_ = os.Remove(home + "nettoken-" + network)
	}
	if FileExists(home + "secret-" + network) {
		_ = os.Remove(home + "secret-" + network)
	}
	if FileExists(home + "wgkey-" + network) {
		_ = os.Remove(home + "wgkey-" + network)
	}
	if FileExists(home + "nm-" + network + ".conf") {
		_ = os.Remove(home + "nm-" + network + ".conf")
	}

	if ifacename != "" {
		if netclientutils.IsWindows() {
			if err = RemoveWindowsConf(ifacename); err == nil {
				log.Println("removed Windows interface", ifacename)
			}
		} else {
			ipExec, err := exec.LookPath("ip")
			if err != nil {
				return err
			}
			out, err := RunCmd(ipExec + " link del " + ifacename, false)
			dontprint := strings.Contains(out, "does not exist") || strings.Contains(out, "Cannot find device")
			if err != nil && !dontprint {
				log.Println("error running command:",ipExec + " link del " + ifacename)
				log.Println(out)
			}
			if nodecfg.PostDown != "" {
				runcmds := strings.Split(nodecfg.PostDown, "; ")
				_ = RunCmds(runcmds, false)
			}
		}
	}
	return err
}

func HasNetwork(network string) bool {

	if netclientutils.IsWindows() {
		return FileExists(netclientutils.GetNetclientPathSpecific() + "netconfig-" + network)
	}
	return FileExists("/etc/systemd/system/netclient-"+network+".timer") ||
		FileExists(netclientutils.GetNetclientPathSpecific()+"netconfig-"+network)
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
