package daemon

import (
	//"github.com/davecgh/go-spew/spew"

	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/gravitl/netmaker/netclient/ncutils"
)

func SetupSystemDDaemon(network string) error {
	if ncutils.IsWindows() {
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

	if !ncutils.FileExists("/usr/local/bin/netclient") {
		os.Symlink("/etc/netclient/netclient", "/usr/local/bin/netclient")
	}
	if !ncutils.FileExists("/etc/netclient/netclient") {
		_, err = ncutils.Copy(binarypath, "/etc/netclient/netclient")
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

OnCalendar=*:*:0/15

[Install]
WantedBy=timers.target
`

	servicebytes := []byte(systemservice)
	timerbytes := []byte(systemtimer)

	if !ncutils.FileExists("/etc/systemd/system/netclient@.service") {
		err = ioutil.WriteFile("/etc/systemd/system/netclient@.service", servicebytes, 0644)
		if err != nil {
			log.Println(err)
			return err
		}
	}

	if !ncutils.FileExists("/etc/systemd/system/netclient-" + network + ".timer") {
		err = ioutil.WriteFile("/etc/systemd/system/netclient-"+network+".timer", timerbytes, 0644)
		if err != nil {
			log.Println(err)
			return err
		}
	}

	_, _ = ncutils.RunCmd("systemctl enable netclient@.service", true)
	_, _ = ncutils.RunCmd("systemctl daemon-reload", true)
	_, _ = ncutils.RunCmd("systemctl enable netclient-"+network+".timer", true)
	_, _ = ncutils.RunCmd("systemctl start netclient-"+network+".timer", true)
	return nil
}

func RemoveSystemDServices(network string) error {
	//sysExec, err := exec.LookPath("systemctl")
	if !ncutils.IsWindows() {
		fullremove, err := isOnlyService(network)
		if err != nil {
			log.Println(err)
		}

		if fullremove {
			_, err = ncutils.RunCmd("systemctl disable netclient@.service", true)
		}
		_, _ = ncutils.RunCmd("systemctl daemon-reload", true)

		if ncutils.FileExists("/etc/systemd/system/netclient-" + network + ".timer") {
			_, _ = ncutils.RunCmd("systemctl disable netclient-"+network+".timer", true)
		}
		if fullremove {
			if ncutils.FileExists("/etc/systemd/system/netclient@.service") {
				err = os.Remove("/etc/systemd/system/netclient@.service")
			}
		}
		if ncutils.FileExists("/etc/systemd/system/netclient-" + network + ".timer") {
			err = os.Remove("/etc/systemd/system/netclient-" + network + ".timer")
		}
		if err != nil {
			log.Println("Error removing file. Please investigate.")
			log.Println(err)
		}
		_, _ = ncutils.RunCmd("systemctl daemon-reload", true)
		_, _ = ncutils.RunCmd("systemctl reset-failed", true)
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
