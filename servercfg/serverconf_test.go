package servercfg

import (
	"testing"

	"github.com/matryer/is"
)

func TestValidateDomain(t *testing.T) {

	t.Run("", func(t *testing.T) {
		is := is.New(t)
		valid := validateDomain("netmaker.hosted")
		is.Equal(valid, true)
	})

	t.Run("", func(t *testing.T) {
		is := is.New(t)
		valid := validateDomain("ipv4test1.hosted")
		is.Equal(valid, true)
	})

	t.Run("", func(t *testing.T) {
		is := is.New(t)
		valid := validateDomain("ip_4?")
		is.Equal(valid, false)
	})

}

func TestGetTcpProxyPublicPort(t *testing.T) {
	t.Setenv("TCP_PROXY_PUBLIC_PORT", "")
	if got := GetTcpProxyPublicPort(); got != 443 {
		t.Fatalf("default: got %d", got)
	}
	t.Setenv("TCP_PROXY_PUBLIC_PORT", "8443")
	if got := GetTcpProxyPublicPort(); got != 8443 {
		t.Fatalf("override: got %d", got)
	}
	t.Setenv("TCP_PROXY_PUBLIC_PORT", "0")
	if got := GetTcpProxyPublicPort(); got != 443 {
		t.Fatalf("invalid 0: got %d", got)
	}
	t.Setenv("TCP_PROXY_PUBLIC_PORT", "nope")
	if got := GetTcpProxyPublicPort(); got != 443 {
		t.Fatalf("invalid string: got %d", got)
	}
}
