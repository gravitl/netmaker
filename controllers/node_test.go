package controller

import (
	"testing"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
)

func TestCreateEgressGateway(t *testing.T) {
	var gateway models.EgressGatewayRequest
	gateway.Interface = "eth0"
	gateway.Ranges = []string{"10.100.100.0/24"}
	database.InitializeDatabase()
	deleteAllNetworks()
	createNet()
	t.Run("NoNodes", func(t *testing.T) {
		node, err := logic.CreateEgressGateway(gateway)
		assert.Equal(t, models.Node{}, node)
		assert.EqualError(t, err, "could not find any records")
	})
	t.Run("Success", func(t *testing.T) {
		testnode := createTestNode()
		gateway.NetID = "skynet"
		gateway.NodeID = testnode.ID

		node, err := logic.CreateEgressGateway(gateway)
		assert.Nil(t, err)
		assert.Equal(t, "yes", node.IsEgressGateway)
		assert.Equal(t, gateway.Ranges, node.EgressGatewayRanges)
	})

}
func TestDeleteEgressGateway(t *testing.T) {
	var gateway models.EgressGatewayRequest
	database.InitializeDatabase()
	deleteAllNetworks()
	createNet()
	createTestNode()
	testnode := createTestNode()
	gateway.Interface = "eth0"
	gateway.Ranges = []string{"10.100.100.0/24"}
	gateway.NetID = "skynet"
	gateway.NodeID = testnode.ID
	t.Run("Success", func(t *testing.T) {
		node, err := logic.CreateEgressGateway(gateway)
		assert.Nil(t, err)
		assert.Equal(t, "yes", node.IsEgressGateway)
		assert.Equal(t, []string{"10.100.100.0/24"}, node.EgressGatewayRanges)
		node, err = logic.DeleteEgressGateway(gateway.NetID, gateway.NodeID)
		assert.Nil(t, err)
		assert.Equal(t, "no", node.IsEgressGateway)
		assert.Equal(t, []string([]string{}), node.EgressGatewayRanges)
		assert.Equal(t, "", node.PostUp)
		assert.Equal(t, "", node.PostDown)
	})
	t.Run("NotGateway", func(t *testing.T) {
		node, err := logic.DeleteEgressGateway(gateway.NetID, gateway.NodeID)
		assert.Nil(t, err)
		assert.Equal(t, "no", node.IsEgressGateway)
		assert.Equal(t, []string([]string{}), node.EgressGatewayRanges)
		assert.Equal(t, "", node.PostUp)
		assert.Equal(t, "", node.PostDown)
	})
	t.Run("BadNode", func(t *testing.T) {
		node, err := logic.DeleteEgressGateway(gateway.NetID, "01:02:03")
		assert.EqualError(t, err, "no result found")
		assert.Equal(t, models.Node{}, node)
		deleteAllNodes()
	})
}

func TestGetNetworkNodes(t *testing.T) {
	database.InitializeDatabase()
	deleteAllNetworks()
	createNet()
	t.Run("BadNet", func(t *testing.T) {
		node, err := logic.GetNetworkNodes("badnet")
		assert.Nil(t, err)
		assert.Equal(t, []models.Node{}, node)
		//assert.Equal(t, "mongo: no documents in result", err.Error())
	})
	t.Run("NoNodes", func(t *testing.T) {
		node, err := logic.GetNetworkNodes("skynet")
		assert.Nil(t, err)
		assert.Equal(t, []models.Node{}, node)
	})
	t.Run("Success", func(t *testing.T) {
		createTestNode()
		node, err := logic.GetNetworkNodes("skynet")
		assert.Nil(t, err)
		assert.NotEqual(t, []models.Node(nil), node)
	})

}
func TestUncordonNode(t *testing.T) {
	database.InitializeDatabase()
	deleteAllNetworks()
	createNet()
	node := createTestNode()
	t.Run("BadID", func(t *testing.T) {
		resp, err := logic.UncordonNode("blahblah")
		assert.Equal(t, models.Node{}, resp)
		assert.EqualError(t, err, "no result found")
	})
	t.Run("Success", func(t *testing.T) {
		resp, err := logic.UncordonNode(node.ID)
		assert.Nil(t, err)
		assert.Equal(t, "no", resp.IsPending)
	})

}
func TestValidateEgressGateway(t *testing.T) {
	var gateway models.EgressGatewayRequest
	t.Run("EmptyRange", func(t *testing.T) {
		gateway.Interface = "eth0"
		gateway.Ranges = []string{}
		err := logic.ValidateEgressGateway(gateway)
		assert.EqualError(t, err, "IP Ranges Cannot Be Empty")
	})
	t.Run("EmptyInterface", func(t *testing.T) {
		gateway.Interface = ""
		err := logic.ValidateEgressGateway(gateway)
		assert.NotNil(t, err)
		assert.Equal(t, "Interface cannot be empty", err.Error())
	})
	t.Run("Success", func(t *testing.T) {
		gateway.Interface = "eth0"
		gateway.Ranges = []string{"10.100.100.0/24"}
		err := logic.ValidateEgressGateway(gateway)
		assert.Nil(t, err)
	})
}

func deleteAllNodes() {
	nodes, _ := logic.GetAllNodes()
	for _, node := range nodes {
		logic.DeleteNodeByID(&node, true)
	}
}

func createTestNode() *models.Node {
	createnode := models.Node{PublicKey: "DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=", Name: "testnode", Endpoint: "10.0.0.1", MacAddress: "01:02:03:04:05:06", Password: "password", Network: "skynet"}
	logic.CreateNode(&createnode)
	return &createnode
}
