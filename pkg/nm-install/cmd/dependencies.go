package cmd

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/bitfield/script"
	"github.com/pterm/pterm"
)

var (
	distro          string
	dockerRequired  bool
	composeRequired bool
)

func installDependencies() {
	var err error
	distro, err = getDistro()
	if err != nil {
		if err.Error() == "unsupported distribution" {
			pterm.Println("unable to install dependencies")
			pterm.Println("you may be using an ", err.Error())
			pterm.Println("to install netmaker, first install docker, docker-compose, and wireguard-tools")
			pterm.Println("and then re-run this program")
			os.Exit(1)
		} else {
			pterm.Println("this does not appear to be a linux OS, cannot proceed")
			os.Exit(2)
		}
	}
	pterm.Println("checking if docker/docker-compose is installed", distro)
	_, err = exec.LookPath("docker")
	if err != nil {
		dockerRequired = true
	}
	_, err = exec.LookPath("docker-compose")
	if err != nil {
		composeRequired = true
	}

	if dockerRequired || composeRequired {
		if distro != "ubuntu" && distro != "debian" {
			installDockerCE(distro)
		} else {
			installDocker()
		}
	}
}

func getDistro() (string, error) {
	id, err := script.File("/etc/os-release").Match("ID").String()
	if err != nil {
		return "", err
	}
	// the order in which these are checked is important
	if strings.Contains(id, "ubuntu") {
		return "ubuntu", nil
	}
	if strings.Contains(id, "debian") {
		return "debian", nil
	}
	if strings.Contains(id, "centos") {
		return "centos", nil
	}
	if strings.Contains(id, "rhel") {
		return "rhel", nil
	}
	if strings.Contains(id, "fedora") {
		return "fedora", nil
	}
	if strings.Contains(id, "alpine") {
		return "alpine", nil
	}
	return "", errors.New("unsupported distrobution")
}

func installDockerCE(distro string) {
	switch distro {
	case "centos":
		_, err := script.Exec("yum install -y yum-utils").Stdout()
		if err != nil {
			panic(err)
		}
		_, err = script.Exec("yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo").Stdout()
		if err != nil {
			panic(err)
		}
		_, err = script.Exec("yum install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin").Stdout()
		if err != nil {
			panic(err)
		}
	case "rhel":
		_, err := script.Exec("yum install -y yum-utils").Stdout()
		if err != nil {
			panic(err)
		}
		_, err = script.Exec("yum-config-manager --add-repo https://download.docker.com/linux/rhel/docker-ce.repo").Stdout()
		if err != nil {
			panic(err)
		}
		_, err = script.Exec("yum install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin").Stdout()
		if err != nil {
			panic(err)
		}
	case "fedora":
		_, err := script.Exec("dnf install -y dnf-plugins-core").Stdout()
		if err != nil {
			panic(err)
		}
		_, err = script.Exec("dnf-config-manager --add-repo https://download.docker.com/linux/fedora/docker-ce.repo").Stdout()
		if err != nil {
			panic(err)
		}
		_, err = script.Exec("dnf install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin").Stdout()
		if err != nil {
			panic(err)
		}
	default:
		panic(errors.New("unsupported distribution"))
	}
	_, err := script.Exec("systemctl start docker").Stdout()
	if err != nil {
		panic(err)
	}
}

func installDocker() {
	_, err := script.Exec("apt-get update").Stdout()
	if err != nil {
		panic(err)
	}
	_, err = script.Exec("apt-get -y install docker docker-compose").Stdout()
	if err != nil {
		panic(err)
	}
}
