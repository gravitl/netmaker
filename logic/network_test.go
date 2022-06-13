package logic

import (
	"testing"

	"github.com/gravitl/netmaker/netclient/ncutils"
	"github.com/stretchr/testify/assert"
)

func TestCheckOverlap(t *testing.T) {
	_, err := ncutils.RunCmd("ip link add nm-0 type wireguard", false)
	assert.Nil(t, err)
	_, err = ncutils.RunCmd("ip a add 10.0.255.254/16 dev nm-0", false)
	assert.Nil(t, err)
	_, err = ncutils.RunCmd("ip -6 a add 2001:db8::/64 dev nm-0", false)
	assert.Nil(t, err)
	t.Run("4Good", func(t *testing.T) {
		err = CheckOverlap("10.10.10.0/24", "")
		assert.Nil(t, err)
	})
	t.Run("4Bad", func(t *testing.T) {
		err = CheckOverlap("10.0.1.0/24", "")
		assert.NotNil(t, err)
	})
	t.Run("6Good", func(t *testing.T) {
		err = CheckOverlap("", "3001:fe8::/64")
		assert.Nil(t, err)
	})
	t.Run("6Bad", func(t *testing.T) {
		err = CheckOverlap("", "2001:db8::1:0/64")
		assert.NotNil(t, err)
	})
	_, err = ncutils.RunCmd("ip link del nm-0", false)

}
