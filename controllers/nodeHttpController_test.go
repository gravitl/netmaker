package controller

import (
	"testing"
	"time"

	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
)

func TestCheckIn(t *testing.T) {
	deleteNet(t)
	createNet()
	node := createTestNode(t)
	time.Sleep(time.Second * 1)
	t.Run("BadNet", func(t *testing.T) {
		resp, err := CheckIn("badnet", node.MacAddress)
		assert.NotNil(t, err)
		assert.Equal(t, models.Node{}, resp)
		assert.Equal(t, "mongo: no documents in result", err.Error())
	})
	t.Run("BadMac", func(t *testing.T) {
		resp, err := CheckIn("skynet", "01:02:03")
		assert.NotNil(t, err)
		assert.Equal(t, models.Node{}, resp)
		assert.Equal(t, "mongo: no documents in result", err.Error())
	})
	t.Run("Success", func(t *testing.T) {
		resp, err := CheckIn("skynet", node.MacAddress)
		assert.Nil(t, err)
		assert.Greater(t, resp.LastCheckIn, node.LastCheckIn)
	})
}
func TestCreateEgressGateway(t *testing.T) {
	var gateway models.EgressGatewayRequest
	gateway.Interface = "eth0"
	gateway.Ranges = ["10.100.100.0/24"]
	deleteNet(t)
	createNet()
	t.Run("NoNodes", func(t *testing.T) {
		node, err := CreateEgressGateway(gateway)
		assert.NotNil(t, err)
		assert.Equal(t, models.Node{}, node)
		assert.Equal(t, "mongo: no documents in result", err.Error())
	})
	t.Run("Success", func(t *testing.T) {
		testnode := createTestNode(t)
		gateway.NetID = "skynet"
		gateway.NodeID = testnode.MacAddress

		node, err := CreateEgressGateway(gateway)
		assert.Nil(t, err)
		assert.Equal(t, true, node.IsEgressGateway)
		assert.Equal(t, "10.100.100.0/24", node.EgressGatewayRange)
	})

}
func TestDeleteEgressGateway(t *testing.T) {
	var gateway models.EgressGatewayRequest
	deleteNet(t)
	createNet()
	createTestNode(t)
	testnode := createTestNode(t)
	gateway.Interface = "eth0"
	gateway.Ranges = ["10.100.100.0/24"]
	gateway.NetID = "skynet"
	gateway.NodeID = testnode.MacAddress
	t.Run("Success", func(t *testing.T) {
		node, err := CreateEgressGateway(gateway)
		assert.Nil(t, err)
		assert.Equal(t, true, node.IsEgressGateway)
		assert.Equal(t, ["10.100.100.0/24"], node.EgressGatewayRanges)
		node, err = DeleteEgressGateway(gateway.NetID, gateway.NodeID)
		assert.Nil(t, err)
		assert.Equal(t, false, node.IsEgressGateway)
		assert.Equal(t, "", node.EgressGatewayRanges)
		assert.Equal(t, "", node.PostUp)
		assert.Equal(t, "", node.PostDown)
	})
	t.Run("NotGateway", func(t *testing.T) {
		node, err := DeleteEgressGateway(gateway.NetID, gateway.NodeID)
		assert.Nil(t, err)
		assert.Equal(t, false, node.IsEgressGateway)
		assert.Equal(t, "", node.EgressGatewayRanges)
		assert.Equal(t, "", node.PostUp)
		assert.Equal(t, "", node.PostDown)
	})
	t.Run("BadNode", func(t *testing.T) {
		node, err := DeleteEgressGateway(gateway.NetID, "01:02:03")
		assert.NotNil(t, err)
		assert.Equal(t, "mongo: no documents in result", err.Error())
		assert.Equal(t, models.Node{}, node)
	})
	t.Run("BadNet", func(t *testing.T) {
		node, err := DeleteEgressGateway("badnet", gateway.NodeID)
		assert.NotNil(t, err)
		assert.Equal(t, "mongo: no documents in result", err.Error())
		assert.Equal(t, models.Node{}, node)
	})

}
func TestGetLastModified(t *testing.T) {
	deleteNet(t)
	createNet()
	createTestNode(t)
	t.Run("BadNet", func(t *testing.T) {
		network, err := GetLastModified("badnet")
		assert.NotNil(t, err)
		assert.Equal(t, models.Network{}, network)
		assert.Equal(t, "mongo: no documents in result", err.Error())
	})
	t.Run("Success", func(t *testing.T) {
		network, err := GetLastModified("skynet")
		assert.Nil(t, err)
		assert.NotEqual(t, models.Network{}, network)
	})
}
func TestGetNetworkNodes(t *testing.T) {
	deleteNet(t)
	createNet()
	t.Run("BadNet", func(t *testing.T) {
		node, err := GetNetworkNodes("badnet")
		assert.Nil(t, err)
		assert.Equal(t, []models.Node(nil), node)
		//assert.Equal(t, "mongo: no documents in result", err.Error())
	})
	t.Run("NoNodes", func(t *testing.T) {
		node, err := GetNetworkNodes("skynet")
		assert.Nil(t, err)
		assert.Equal(t, []models.Node(nil), node)
	})
	t.Run("Success", func(t *testing.T) {
		createTestNode(t)
		node, err := GetNetworkNodes("skynet")
		assert.Nil(t, err)
		assert.NotEqual(t, []models.Node(nil), node)
	})

}
func TestUncordonNode(t *testing.T) {
	deleteNet(t)
	createNet()
	node := createTestNode(t)
	t.Run("BadNet", func(t *testing.T) {
		resp, err := UncordonNode("badnet", node.MacAddress)
		assert.NotNil(t, err)
		assert.Equal(t, models.Node{}, resp)
		assert.Equal(t, "mongo: no documents in result", err.Error())
	})
	t.Run("BadMac", func(t *testing.T) {
		resp, err := UncordonNode("skynet", "01:02:03")
		assert.NotNil(t, err)
		assert.Equal(t, models.Node{}, resp)
		assert.Equal(t, "mongo: no documents in result", err.Error())
	})
	t.Run("Success", func(t *testing.T) {
		resp, err := CheckIn("skynet", node.MacAddress)
		assert.Nil(t, err)
		assert.Equal(t, false, resp.IsPending)
	})

}
func TestValidateEgressGateway(t *testing.T) {
	var gateway models.EgressGatewayRequest
	t.Run("EmptyRange", func(t *testing.T) {
		gateway.Interface = "eth0"
		gateway.Ranges = []string{}
		err := ValidateEgressGateway(gateway)
		assert.NotNil(t, err)
		assert.Equal(t, "IP Range Not Valid", err.Error())
	})
	t.Run("EmptyInterface", func(t *testing.T) {
		gateway.Interface = ""
		err := ValidateEgressGateway(gateway)
		assert.NotNil(t, err)
		assert.Equal(t, "Interface cannot be empty", err.Error())
	})
	t.Run("Success", func(t *testing.T) {
		gateway.Interface = "eth0"
		gateway.Ranges = ["10.100.100.0/24"]
		err := ValidateEgressGateway(gateway)
		assert.Nil(t, err)
	})
}

//func TestUpdateNode(t *testing.T) {
//}
