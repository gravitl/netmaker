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
func TestCreateGateway(t *testing.T) {
	var gateway models.GatewayRequest
	gateway.Interface = "eth0"
	gateway.RangeString = "10.100.100.0/24"
	deleteNet(t)
	createNet()
	t.Run("NoNodes", func(t *testing.T) {
		node, err := CreateGateway(gateway)
		assert.NotNil(t, err)
		assert.Equal(t, models.Node{}, node)
		assert.Equal(t, "mongo: no documents in result", err.Error())
	})
	t.Run("Success", func(t *testing.T) {
		testnode := createTestNode(t)
		gateway.NetID = "skynet"
		gateway.NodeID = testnode.MacAddress

		node, err := CreateGateway(gateway)
		assert.Nil(t, err)
		assert.Equal(t, true, node.IsGateway)
		assert.Equal(t, "10.100.100.0/24", node.GatewayRange)
	})

}
func TestDeleteGateway(t *testing.T) {
	var gateway models.GatewayRequest
	deleteNet(t)
	createNet()
	createTestNode(t)
	testnode := createTestNode(t)
	gateway.Interface = "eth0"
	gateway.RangeString = "10.100.100.0/24"
	gateway.NetID = "skynet"
	gateway.NodeID = testnode.MacAddress
	t.Run("Success", func(t *testing.T) {
		node, err := CreateGateway(gateway)
		assert.Nil(t, err)
		assert.Equal(t, true, node.IsGateway)
		assert.Equal(t, "10.100.100.0/24", node.GatewayRange)
		node, err = DeleteGateway(gateway.NetID, gateway.NodeID)
		assert.Nil(t, err)
		assert.Equal(t, false, node.IsGateway)
		assert.Equal(t, "", node.GatewayRange)
		assert.Equal(t, "", node.PostUp)
		assert.Equal(t, "", node.PostDown)
	})
	t.Run("NotGateway", func(t *testing.T) {
		node, err := DeleteGateway(gateway.NetID, gateway.NodeID)
		assert.Nil(t, err)
		assert.Equal(t, false, node.IsGateway)
		assert.Equal(t, "", node.GatewayRange)
		assert.Equal(t, "", node.PostUp)
		assert.Equal(t, "", node.PostDown)
	})
	t.Run("BadNode", func(t *testing.T) {
		node, err := DeleteGateway(gateway.NetID, "01:02:03")
		assert.NotNil(t, err)
		assert.Equal(t, "mongo: no documents in result", err.Error())
		assert.Equal(t, models.Node{}, node)
	})
	t.Run("BadNet", func(t *testing.T) {
		node, err := DeleteGateway("badnet", gateway.NodeID)
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
		assert.Equal(t, []models.ReturnNode(nil), node)
		//assert.Equal(t, "mongo: no documents in result", err.Error())
	})
	t.Run("NoNodes", func(t *testing.T) {
		node, err := GetNetworkNodes("skynet")
		assert.Nil(t, err)
		assert.Equal(t, []models.ReturnNode(nil), node)
	})
	t.Run("Success", func(t *testing.T) {
		createTestNode(t)
		node, err := GetNetworkNodes("skynet")
		assert.Nil(t, err)
		assert.NotEqual(t, []models.ReturnNode(nil), node)
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
func TestValidateGateway(t *testing.T) {
	var gateway models.GatewayRequest
	t.Run("InvalidRange", func(t *testing.T) {
		gateway.Interface = "eth0"
		gateway.RangeString = "helloworld"
		err := ValidateGateway(gateway)
		assert.NotNil(t, err)
		assert.Equal(t, "IP Range Not Valid", err.Error())
	})
	t.Run("EmptyRange", func(t *testing.T) {
		gateway.Interface = "eth0"
		gateway.RangeString = ""
		err := ValidateGateway(gateway)
		assert.NotNil(t, err)
		assert.Equal(t, "IP Range Not Valid", err.Error())
	})
	t.Run("EmptyInterface", func(t *testing.T) {
		gateway.Interface = ""
		err := ValidateGateway(gateway)
		assert.NotNil(t, err)
		assert.Equal(t, "Interface cannot be empty", err.Error())
	})
	t.Run("Success", func(t *testing.T) {
		gateway.Interface = "eth0"
		gateway.RangeString = "10.100.100.0/24"
		err := ValidateGateway(gateway)
		assert.Nil(t, err)
	})
}

//func TestUpdateNode(t *testing.T) {
//}
