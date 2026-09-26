package logic

import (
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/stretchr/testify/assert"
)

func TestIsUserOwnedHost(t *testing.T) {
	assert.False(t, IsUserOwnedHost(nil))
	assert.False(t, IsUserOwnedHost(&schema.Host{Name: "server"}))
	assert.True(t, IsUserOwnedHost(&schema.Host{Name: "laptop", OwnerUsername: "alice"}))
}

func TestNodeOwnerUsername(t *testing.T) {
	assert.Equal(t, "", NodeOwnerUsername(nil))

	hostOwned := &models.Node{OwnerID: "alice"}
	assert.Equal(t, "alice", NodeOwnerUsername(hostOwned))
	assert.True(t, IsUserOwnedDevice(hostOwned))

	legacy := &models.Node{
		IsStatic:   true,
		IsUserNode: true,
		StaticNode: models.ExtClient{OwnerID: "bob", RemoteAccessClientID: "mac"},
	}
	assert.Equal(t, "bob", NodeOwnerUsername(legacy))
	assert.False(t, IsUserOwnedDevice(legacy))

	unowned := &models.Node{CommonNode: models.CommonNode{ID: uuid.New()}}
	assert.Equal(t, "", NodeOwnerUsername(unowned))
	assert.False(t, IsUserOwnedDevice(unowned))
}

func TestIsUserOwnedDevice_requiresHostBackedOwner(t *testing.T) {
	n := &models.Node{
		IsStatic: false,
		OwnerID:  "carol",
		StaticNode: models.ExtClient{
			OwnerID:    "carol",
			DeviceName: "carol-laptop",
		},
	}
	assert.True(t, IsUserOwnedDevice(n))

	// API-shaped ExtClient must not be treated as a host-backed device.
	n.IsUserNode = true
	n.IsStatic = true
	assert.False(t, IsUserOwnedDevice(n))
}
