package ips_test

import (
	"testing"

	"github.com/gravitl/netmaker/logic/ips"
	"github.com/stretchr/testify/assert"
)

func TestIp4(t *testing.T) {
	const ipv4Cidr = "192.168.0.0/16"
	const ipv6Cidr = "fde6:be04:fa5e:d076::/64"
	//delete all current users
	t.Run("Valid Ipv4", func(t *testing.T) {
		_, err := ips.GetFirstAddr(ipv4Cidr)
		assert.Nil(t, err)
	})
	t.Run("Invalid Ipv4", func(t *testing.T) {
		_, err := ips.GetFirstAddr(ipv6Cidr)
		assert.NotNil(t, err)
	})
	t.Run("Valid IPv6", func(t *testing.T) {
		_, err := ips.GetFirstAddr6(ipv6Cidr)
		assert.Nil(t, err)
	})
	t.Run("Invalid IPv6", func(t *testing.T) {
		_, err := ips.GetFirstAddr6(ipv4Cidr)
		assert.NotNil(t, err)
	})
	t.Run("Last IPv4", func(t *testing.T) {
		addr, err := ips.GetLastAddr(ipv4Cidr)
		assert.Nil(t, err)
		assert.Equal(t, addr.GetNetIPAddr().IP.String(), "192.168.255.254")
	})
	t.Run("First IPv4", func(t *testing.T) {
		addr, err := ips.GetFirstAddr(ipv4Cidr)
		assert.Nil(t, err)
		assert.Equal(t, addr.GetNetIPAddr().IP.String(), "192.168.0.1")
	})
	t.Run("Last IPv6", func(t *testing.T) {
		last, err := ips.GetLastAddr6(ipv6Cidr)
		assert.Nil(t, err)
		assert.Equal(t, last.GetNetIPAddr().IP.String(), "fde6:be04:fa5e:d076:ffff:ffff:ffff:ffff")
	})
	t.Run("First IPv6", func(t *testing.T) {
		first, err := ips.GetFirstAddr6(ipv6Cidr)
		assert.Nil(t, err)
		assert.Equal(t, first.GetNetIPAddr().IP.String(), "fde6:be04:fa5e:d076::")
	})
}
