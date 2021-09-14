package controller

import (
	"testing"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
)

func TestGetPeerList(t *testing.T) {
	database.InitializeDatabase()
	deleteAllNetworks()
	createNet()
	t.Run("NoNodes", func(t *testing.T) {
		peers, err := GetPeersList("skynet", false)
		assert.Nil(t, err)
		assert.Nil(t, peers)
	})
	node := createTestNode()
	t.Run("One Node", func(t *testing.T) {
		peers, err := GetPeersList("skynet", false)
		assert.Nil(t, err)
		assert.Equal(t, node.Address, peers[0].Address)
	})
	t.Run("Multiple Nodes", func(t *testing.T) {
		createnode := models.Node{PublicKey: "RM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=", Endpoint: "10.0.0.2", MacAddress: "02:02:03:04:05:06", Password: "password", Network: "skynet"}
		CreateNode(createnode, "skynet")
		peers, err := GetPeersList("skynet", false)
		assert.Nil(t, err)
		assert.Equal(t, len(peers), 2)
		foundNodeEndpoint := false
		for _, peer := range peers {
			if foundNodeEndpoint = peer.Endpoint == createnode.Endpoint; foundNodeEndpoint {
				break
			}
		}
		assert.True(t, foundNodeEndpoint)
	})
}

func TestDeleteNode(t *testing.T) {
	database.InitializeDatabase()
	deleteAllNetworks()
	createNet()
	node := createTestNode()
	t.Run("NodeExists", func(t *testing.T) {
		err := DeleteNode(node.MacAddress, true)
		assert.Nil(t, err)
	})
	t.Run("NonExistantNode", func(t *testing.T) {
		err := DeleteNode(node.MacAddress, true)
		assert.Nil(t, err)
	})
}

func TestGetNode(t *testing.T) {
	database.InitializeDatabase()
	deleteAllNetworks()
	t.Run("NoNode", func(t *testing.T) {
		response, err := GetNode("01:02:03:04:05:06", "skynet")
		assert.Equal(t, models.Node{}, response)
		assert.EqualError(t, err, "unexpected end of JSON input")
	})
	createNet()
	node := createTestNode()

	t.Run("NodeExists", func(t *testing.T) {
		response, err := GetNode(node.MacAddress, node.Network)
		assert.Nil(t, err)
		assert.Equal(t, "10.0.0.1", response.Endpoint)
		assert.Equal(t, "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=", response.PublicKey)
		assert.Equal(t, "01:02:03:04:05:06", response.MacAddress)
		assert.Equal(t, int32(51821), response.ListenPort)
		assert.NotNil(t, response.Name)
		assert.Equal(t, "skynet", response.Network)
		assert.Equal(t, "nm-skynet", response.Interface)
	})
	t.Run("BadMac", func(t *testing.T) {
		response, err := GetNode("01:02:03:04:05:07", node.Network)
		assert.Equal(t, models.Node{}, response)
		assert.EqualError(t, err, "unexpected end of JSON input")
	})
	t.Run("BadNetwork", func(t *testing.T) {
		response, err := GetNode(node.MacAddress, "badnet")
		assert.Equal(t, models.Node{}, response)
		assert.EqualError(t, err, "unexpected end of JSON input")
	})

}

func TestCreateNode(t *testing.T) {
	t.Skip()
	database.InitializeDatabase()
	deleteAllNetworks()
	createNet()
	createnode := models.Node{PublicKey: "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=", Endpoint: "10.0.0.1", MacAddress: "01:02:03:04:05:06", Password: "password", Network: "skynet"}
	//err := ValidateNodeCreate("skynet", createnode)
	//assert.Nil(t, err)
	node, err := CreateNode(createnode, "skynet")
	assert.Nil(t, err)
	assert.Equal(t, "10.0.0.1", node.Endpoint)
	assert.Equal(t, "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=", node.PublicKey)
	assert.Equal(t, "01:02:03:04:05:06", node.MacAddress)
	assert.Equal(t, int32(51821), node.ListenPort)
	assert.NotNil(t, node.Name)
	assert.Equal(t, "skynet", node.Network)
	assert.Equal(t, "nm-skynet", node.Interface)
}

func TestSetNetworkNodesLastModified(t *testing.T) {
	database.InitializeDatabase()
	deleteAllNetworks()
	createNet()
	t.Run("InvalidNetwork", func(t *testing.T) {
		err := SetNetworkNodesLastModified("badnet")
		assert.EqualError(t, err, "no result found")
	})
	t.Run("NetworkExists", func(t *testing.T) {
		err := SetNetworkNodesLastModified("skynet")
		assert.Nil(t, err)
	})
}

func createTestNode() models.Node {
	createnode := models.Node{PublicKey: "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=", Endpoint: "10.0.0.1", MacAddress: "01:02:03:04:05:06", Password: "password", Network: "skynet"}
	node, _ := CreateNode(createnode, "skynet")
	return node
}
