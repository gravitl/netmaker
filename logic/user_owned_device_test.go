package logic

import (
	"context"
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

func TestUserDevicesAreNotPeers(t *testing.T) {
	alice := &schema.Host{ID: uuid.New(), OwnerUsername: "alice"}
	bob := &schema.Host{ID: uuid.New(), OwnerUsername: "bob"}
	gateway := &schema.Host{ID: uuid.New()}
	assert.True(t, UserDevicesAreNotPeers(alice, bob))
	assert.False(t, UserDevicesAreNotPeers(alice, gateway))
	assert.False(t, UserDevicesAreNotPeers(alice, alice))
	assert.False(t, UserDevicesAreNotPeers(gateway, nil))
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

func TestNodeFlowIdentity(t *testing.T) {
	node := &models.Node{CommonNode: models.CommonNode{ID: uuid.New()}}

	server := &schema.Host{Name: "server"}
	assert.Equal(t, models.PeerIdentity{
		ID:   node.ID.String(),
		Type: models.PeerType_Node,
		Name: "server",
	}, nodeFlowIdentity(node, server))

	laptop := &schema.Host{Name: "laptop", OwnerUsername: "alice"}
	assert.Equal(t, models.PeerIdentity{
		ID:   "alice",
		Type: models.PeerType_User,
		Name: "alice",
	}, nodeFlowIdentity(node, laptop))
}

func TestPeerAllowedSkipsResourcePolicyForUserDevices(t *testing.T) {
	orig := IsUserAllowedToCommunicate
	t.Cleanup(func() { IsUserAllowedToCommunicate = orig })

	userDev := models.Node{OwnerID: "alice"}
	infra := models.Node{}

	IsUserAllowedToCommunicate = func(context.Context, string, models.Node) (bool, []models.Acl) {
		return false, nil
	}
	if PeerAllowed(context.Background(), userDev, infra, true) {
		t.Fatal("default resource policy must not permit a user device")
	}

	IsUserAllowedToCommunicate = func(_ context.Context, userName string, _ models.Node) (bool, []models.Acl) {
		return userName == "alice", nil
	}
	if !PeerAllowed(context.Background(), userDev, infra, true) {
		t.Fatal("user policy should allow the user device")
	}
	if !PeerAllowed(context.Background(), infra, infra, true) {
		t.Fatal("resource default policy should still allow infrastructure peers")
	}
}

func TestErrUserOwnedInfrastructureRole(t *testing.T) {
	assert.NoError(t, ErrUserOwnedNodeInfrastructureRole(nil))
	assert.NoError(t, ErrUserOwnedHostInfrastructureRole(nil))
	assert.NoError(t, ErrUserOwnedNodeInfrastructureRole(&models.Node{}))
	assert.NoError(t, ErrUserOwnedHostInfrastructureRole(&schema.Host{Name: "gw"}))

	err := ErrUserOwnedNodeInfrastructureRole(&models.Node{OwnerID: "alice"})
	assert.ErrorIs(t, err, ErrUserDeviceInfrastructureRole)
	err = ErrUserOwnedHostInfrastructureRole(&schema.Host{OwnerUsername: "alice"})
	assert.ErrorIs(t, err, ErrUserDeviceInfrastructureRole)

	legacy := &models.Node{IsStatic: true, IsUserNode: true, OwnerID: "bob"}
	assert.NoError(t, ErrUserOwnedNodeInfrastructureRole(legacy))
}

func TestErrUserDeviceGainingInfrastructureRole(t *testing.T) {
	ctx := context.Background()
	current := &models.Node{OwnerID: "alice"}

	cases := make([]*models.Node, 6)
	for i := range cases {
		cases[i] = &models.Node{OwnerID: "alice"}
	}
	cases[0].IsGw = true
	cases[1].IsIngressGateway = true
	cases[2].IsRelay = true
	cases[3].IsInternetGateway = true
	cases[4].IsAutoRelay = true
	cases[5].EgressDetails.IsEgressGateway = true
	for _, next := range cases {
		assert.ErrorIs(t, ErrUserDeviceGainingInfrastructureRole(ctx, current, next), ErrUserDeviceInfrastructureRole)
	}

	unchanged := &models.Node{OwnerID: "alice"}
	assert.NoError(t, ErrUserDeviceGainingInfrastructureRole(ctx, current, unchanged))

	alreadyGateway := &models.Node{OwnerID: "alice"}
	alreadyGateway.IsGw = true
	stillGateway := &models.Node{OwnerID: "alice"}
	stillGateway.IsGw = true
	assert.NoError(t, ErrUserDeviceGainingInfrastructureRole(ctx, alreadyGateway, stillGateway))

	infra := &models.Node{}
	gateway := &models.Node{}
	gateway.IsGw = true
	assert.NoError(t, ErrUserDeviceGainingInfrastructureRole(ctx, infra, gateway))
}
