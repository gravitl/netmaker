package logic

import (
	"testing"
)

func TestContainsCIDR(t *testing.T) {

	b := ContainsCIDR("10.1.1.2/32", "10.1.1.0/24")
	if !b {
		t.Errorf("expected true, returned %v", b)
	}

	b = ContainsCIDR("10.1.1.2/32", "10.5.1.0/24")
	if b {
		t.Errorf("expected false, returned %v", b)
	}

	b = ContainsCIDR("fd52:65f5:d685:d11d::1/64", "fd52:65f5:d685:d11d::/64")
	if !b {
		t.Errorf("expected true, returned %v", b)
	}

	b1 := ContainsCIDR("fd10:10::/64", "fd10::/16")
	if !b1 {
		t.Errorf("expected true, returned %v", b1)
	}

	b1 = ContainsCIDR("fd10:10::/64", "fd10::/64")
	if b1 {
		t.Errorf("expected false, returned %v", b1)
	}
}
