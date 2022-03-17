// package for logicing client and server code
package sysctl

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/gravitl/netmaker/logic"
)

type (
	sysctl struct {
		path   string
		config map[string]string
	}
)

const (
	NM_SYSCTL_CONF = "99-nm.conf"
)

var (
	sysctlPaths map[string]string = map[string]string{
		"linux":   "/usr/local/lib/sysctl.d",
		"darwin":  "/usr/local/lib/sysctl.d",
		"freebsd": "/usr/local/lib/sysctl.d",
		"windows": "",
	}
)

func (s *sysctl) set(key, value string) {
	s.config[key] = value
}

func (s *sysctl) get(key string) (val string) {
	val, _ = s.config[key]
	return
}

func (s *sysctl) delete(key string) {
	delete(s.config, key)
}

func (s *sysctl) update() error {
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil || f == nil {
		return err
	}
	defer f.Close()
	for k, v := range s.config {
		ln := []byte(fmt.Sprintf("%s\n", strings.Join([]string{k, v}, "=")))
		if _, err := f.Write(ln); err != nil {
			return err
		}
	}
	return nil
}

func load() (s *sysctl, err error) {
	s = &sysctl{
		path:   path.Join(sysctlPaths[runtime.GOOS], NM_SYSCTL_CONF),
		config: make(map[string]string),
	}

	if !logic.FileExists(s.path) {
		os.MkdirAll(sysctlPaths[runtime.GOOS], os.ModeDir)
	}

	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil {
		return s, err
	}
	defer f.Close()

	for sc := bufio.NewScanner(f); sc.Scan(); {
		line := sc.Bytes()
		if len(line) >= 1 && line[0] == '#' {
			continue
		}
		if kvpair := bytes.Split(line, []byte{'='}); len(kvpair) == 2 {
			s.config[string(kvpair[0])] = string(kvpair[1])
		}
	}
	return
}

func SysctlSetIPForwarding() error {
	conf, err := load()
	if err != nil {
		return err
	}
	conf.set("net.ipv4.ip_forward", "1")
	return conf.update()
}
