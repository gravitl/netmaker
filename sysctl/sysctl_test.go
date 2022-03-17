// package for logicing client and server code
package sysctl

import (
	"testing"
)

func Test_sysctl_load(t *testing.T) {
	sctl, err := load()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(sctl.config)
	sctl.set("net.ipv4.conf.all.secure_redirects", "1")
	if err := sctl.update(); err != nil {
		t.Fatal(err)
	}
	sctl, err = load()
	if val, ok := sctl.config["net.ipv4.conf.all.secure_redirects"]; !ok || val != "1" {
		t.Fatalf("Expected updated value net.ipv4.conf.all.secure_redirects to equal 1, received: exists: %v, val: %s", ok, val)
	}

}
