package logic

import (
	"testing"

	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
)

func TestIsRACExtClient(t *testing.T) {
	assert.True(t, IsRACExtClient(models.ExtClient{DeviceID: "dev-1"}))
	assert.True(t, IsRACExtClient(models.ExtClient{RemoteAccessClientID: "rac-1"}))
	assert.True(t, IsRACExtClient(models.ExtClient{DeviceID: "dev-1", RemoteAccessClientID: "rac-1"}))
	assert.False(t, IsRACExtClient(models.ExtClient{}))
	assert.True(t, IsStaticConfigExtClient(models.ExtClient{}))
	assert.False(t, IsStaticConfigExtClient(models.ExtClient{DeviceID: "dev-1"}))
}

func TestUpdateExtClientPreservesPostUpPostDown(t *testing.T) {
	old := models.ExtClient{
		ClientID: "cfg-1",
		PostUp:   "iptables -A FORWARD -i wg0 -j ACCEPT",
		PostDown: "iptables -D FORWARD -i wg0 -j ACCEPT",
		Enabled:  true,
	}
	// Sparse update (e.g. toggle/rename) with nil PostUp/PostDown must not wipe scripts.
	updated := UpdateExtClient(&old, &models.CustomExtClient{
		ClientID: "cfg-1",
		Enabled:  false,
	})
	assert.Equal(t, old.PostUp, updated.PostUp)
	assert.Equal(t, old.PostDown, updated.PostDown)
	assert.False(t, updated.Enabled)

	postUp := "echo up\r\necho up2"
	postDown := "echo down\r\necho down2"
	updated = UpdateExtClient(&old, &models.CustomExtClient{
		ClientID: "cfg-1",
		Enabled:  true,
		PostUp:   &postUp,
		PostDown: &postDown,
	})
	assert.Equal(t, "echo up\necho up2", updated.PostUp)
	assert.Equal(t, "echo down\necho down2", updated.PostDown)

	empty := ""
	cleared := UpdateExtClient(&old, &models.CustomExtClient{
		ClientID: "cfg-1",
		PostUp:   &empty,
		PostDown: &empty,
	})
	assert.Equal(t, "", cleared.PostUp)
	assert.Equal(t, "", cleared.PostDown)
}
