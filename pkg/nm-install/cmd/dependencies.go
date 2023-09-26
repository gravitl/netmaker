package cmd

import (
	"os"
	"os/exec"
)

type dependecies struct {
	wireguard      bool
	wireguardTools bool
	docker         bool
	dockerCompose  bool
	dependencies   []string
	update         string
	install        string
}

func installDependencies() {
	dep := dependecies{}
	if exists("/etc/debian_version") {
		dep.dependencies = []string{"git", "wireguard", "wireguard-tools", "dnsutils",
			"jq", "docker-io", "docker-compose", "grep", "awk"}
		dep.update = "apt update"
		dep.install = "apt-get install -y"
	} else if exists("/etc/alpine-release") {
		dep.dependencies = []string{"git wireguard jq docker.io docker-compose grep gawk"}
		dep.update = "apk update"
		dep.install = "apk --update add"
	} else if exists("/etc/centos-release") {
		dep.dependencies = []string{"git wireguard jq bind-utils docker.io docker-compose grep gawka"}
		dep.update = "yum update"
		dep.install = "yum install -y"
	} else if exists("/etc/fedora-release") {
		dep.dependencies = []string{"git wireguard bind-utils jq docker.io docker-compose grep gawk"}
		dep.update = "dnf update"
		dep.install = "dnf install -y"
	} else if exists("/etc/redhat-release") {
		dep.dependencies = []string{"git wireguard jq docker.io bind-utils docker-compose grep gawk"}
		dep.update = "yum update"
		dep.install = "yum install -y"
	} else if exists("/etc/arch-release") {
		dep.dependencies = []string{"git wireguard-tools dnsutils jq docker.io docker-compose grep gawk"}
		dep.update = "pacman -Sy"
		dep.install = "pacman -S --noconfirm"
	} else {
		dep.install = ""
	}
	//check if installed
	_, err := exec.LookPath("wg")
	if err != nil {
		dep.wireguardTools = true
		dep.dependencies = append(dep.dependencies, "wireguard-tools")
	}
	_, err = exec.LookPath("docker")
	if err != nil {
		dep.docker = true
		dep.dependencies = append(dep.dependencies, "docker-ce")
	}
	_, err = exec.LookPath("docker-compose")
	if err != nil {
		dep.dockerCompose = true
		dep.dependencies = append(dep.dependencies, "docker-compose")
	}
}

func exists(file string) bool {
	if _, err := os.Stat(file); err != nil {
		return false
	}
	return true
}
