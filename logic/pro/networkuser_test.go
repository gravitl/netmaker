package pro

import (
	"testing"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/models/promodels"
	"github.com/stretchr/testify/assert"
)

func TestNetworkUserLogic(t *testing.T) {
	database.InitializeDatabase()
	networkUser := promodels.NetworkUser{
		ID: "helloworld",
	}
	network := models.Network{
		NetID:        "skynet",
		AddressRange: "192.168.0.0/24",
	}
	nodes := []models.Node{
		models.Node{ID: "coolnode"},
	}

	clients := []models.ExtClient{
		models.ExtClient{
			ClientID: "coolclient",
		},
	}
	AddProNetDefaults(&network)
	t.Run("Net Users initialized successfully", func(t *testing.T) {
		err := InitializeNetworkUsers(network.NetID)
		assert.Nil(t, err)
	})

	t.Run("Error when no network users", func(t *testing.T) {
		user, err := GetNetworkUser(network.NetID, networkUser.ID)
		assert.Nil(t, user)
		assert.NotNil(t, err)
	})

	t.Run("Successful net user create", func(t *testing.T) {
		DeleteNetworkUser(network.NetID, string(networkUser.ID))
		err := CreateNetworkUser(&network, &networkUser)
		assert.Nil(t, err)
		user, err := GetNetworkUser(network.NetID, networkUser.ID)
		assert.NotNil(t, user)
		assert.Nil(t, err)
		assert.Equal(t, 0, user.AccessLevel)
		assert.Equal(t, 0, user.ClientLimit)
	})

	t.Run("Successful net user update", func(t *testing.T) {
		networkUser.AccessLevel = 0
		networkUser.ClientLimit = 1
		err := UpdateNetworkUser(network.NetID, &networkUser)
		assert.Nil(t, err)
		user, err := GetNetworkUser(network.NetID, networkUser.ID)
		assert.NotNil(t, user)
		assert.Nil(t, err)
		assert.Equal(t, 0, user.AccessLevel)
		assert.Equal(t, 1, user.ClientLimit)
	})

	t.Run("Successful net user node isallowed", func(t *testing.T) {
		networkUser.Nodes = append(networkUser.Nodes, "coolnode")
		err := UpdateNetworkUser(network.NetID, &networkUser)
		assert.Nil(t, err)
		isUserNodeAllowed := IsUserNodeAllowed(nodes[:], network.NetID, string(networkUser.ID), "coolnode")
		assert.True(t, isUserNodeAllowed)
	})

	t.Run("Successful net user node not allowed", func(t *testing.T) {
		isUserNodeAllowed := IsUserNodeAllowed(nodes[:], network.NetID, string(networkUser.ID), "notanode")
		assert.False(t, isUserNodeAllowed)
	})

	t.Run("Successful net user client isallowed", func(t *testing.T) {
		networkUser.Clients = append(networkUser.Clients, "coolclient")
		err := UpdateNetworkUser(network.NetID, &networkUser)
		assert.Nil(t, err)
		isUserClientAllowed := IsUserClientAllowed(clients[:], network.NetID, string(networkUser.ID), "coolclient")
		assert.True(t, isUserClientAllowed)
	})

	t.Run("Successful net user client not allowed", func(t *testing.T) {
		isUserClientAllowed := IsUserClientAllowed(clients[:], network.NetID, string(networkUser.ID), "notaclient")
		assert.False(t, isUserClientAllowed)
	})

	t.Run("Successful net user delete", func(t *testing.T) {
		err := DeleteNetworkUser(network.NetID, string(networkUser.ID))
		assert.Nil(t, err)
		user, err := GetNetworkUser(network.NetID, networkUser.ID)
		assert.Nil(t, user)
		assert.NotNil(t, err)
	})
}
