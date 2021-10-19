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
		node, err := CreateEgressGateway(gateway)
		assert.Equal(t, models.Node{}, node)
		assert.EqualError(t, err, "unable to get record key")
	})
	t.Run("Success", func(t *testing.T) {
		testnode := createTestNode()
		gateway.NetID = "skynet"
		gateway.NodeID = testnode.MacAddress

		node, err := CreateEgressGateway(gateway)
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
	gateway.NodeID = testnode.MacAddress
	t.Run("Success", func(t *testing.T) {
		node, err := CreateEgressGateway(gateway)
		assert.Nil(t, err)
		assert.Equal(t, "yes", node.IsEgressGateway)
		assert.Equal(t, []string{"10.100.100.0/24"}, node.EgressGatewayRanges)
		node, err = DeleteEgressGateway(gateway.NetID, gateway.NodeID)
		assert.Nil(t, err)
		assert.Equal(t, "no", node.IsEgressGateway)
		assert.Equal(t, []string([]string{}), node.EgressGatewayRanges)
		assert.Equal(t, "", node.PostUp)
		assert.Equal(t, "", node.PostDown)
	})
	t.Run("NotGateway", func(t *testing.T) {
		node, err := DeleteEgressGateway(gateway.NetID, gateway.NodeID)
		assert.Nil(t, err)
		assert.Equal(t, "no", node.IsEgressGateway)
		assert.Equal(t, []string([]string{}), node.EgressGatewayRanges)
		assert.Equal(t, "", node.PostUp)
		assert.Equal(t, "", node.PostDown)
	})
	t.Run("BadNode", func(t *testing.T) {
		node, err := DeleteEgressGateway(gateway.NetID, "01:02:03")
		assert.EqualError(t, err, "no result found")
		assert.Equal(t, models.Node{}, node)
	})
	t.Run("BadNet", func(t *testing.T) {
		node, err := DeleteEgressGateway("badnet", gateway.NodeID)
		assert.EqualError(t, err, "no result found")
		assert.Equal(t, models.Node{}, node)
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
	t.Run("BadNet", func(t *testing.T) {
		resp, err := UncordonNode("badnet", node.MacAddress)
		assert.Equal(t, models.Node{}, resp)
		assert.EqualError(t, err, "no result found")
	})
	t.Run("BadMac", func(t *testing.T) {
		resp, err := UncordonNode("skynet", "01:02:03")
		assert.Equal(t, models.Node{}, resp)
		assert.EqualError(t, err, "no result found")
	})
	t.Run("Success", func(t *testing.T) {
		resp, err := UncordonNode("skynet", node.MacAddress)
		assert.Nil(t, err)
		assert.Equal(t, "no", resp.IsPending)
	})

}
func TestValidateEgressGateway(t *testing.T) {
	var gateway models.EgressGatewayRequest
	t.Run("EmptyRange", func(t *testing.T) {
		gateway.Interface = "eth0"
		gateway.Ranges = []string{}
		err := ValidateEgressGateway(gateway)
		assert.EqualError(t, err, "IP Ranges Cannot Be Empty")
	})
	t.Run("EmptyInterface", func(t *testing.T) {
		gateway.Interface = ""
		err := ValidateEgressGateway(gateway)
		assert.NotNil(t, err)
		assert.Equal(t, "Interface cannot be empty", err.Error())
	})
	t.Run("Success", func(t *testing.T) {
		gateway.Interface = "eth0"
		gateway.Ranges = []string{"10.100.100.0/24"}
		err := ValidateEgressGateway(gateway)
		assert.Nil(t, err)
	})
}

func TestCreateIngressGateway(t *testing.T) {
	database.InitializeDatabase()
	deleteAllNetworks()
	t.Run("NoNode", func(*testing.T) {
		node, err := CreateIngressGateway("nonet", "01:02:03:04:05:06")
		assert.EqualError(t, err, "could not find any records")
		assert.Equal(t, models.Node{}, node)

	})
	t.Run("BadNet", func(*testing.T) {
		createNet()
		testNode := createTestNode()

		node, err := CreateIngressGateway("badnet", testNode.MacAddress)
		assert.EqualError(t, err, "no result found")
		assert.Equal(t, models.Node{}, node)
	})
	t.Run("Windows", func(*testing.T) {
		node, err := GetNode("01:02:03:04:05:06", "skynet")
		update, err := GetNode("01:02:03:04:05:06", "skynet")
		update.OS = "windows"
		err = node.Update(&update)
		assert.Nil(t, err)
		gateway, err := CreateIngressGateway(node.Network, node.MacAddress)
		assert.EqualError(t, err, "windows is unsupported for ingress gateways")
		assert.Equal(t, models.Node{}, gateway)
	})
	t.Run("MacOs", func(*testing.T) {
		node, err := GetNode("01:02:03:04:05:06", "skynet")
		update, err := GetNode("01:02:03:04:05:06", "skynet")
		update.OS = "macos"
		err = node.Update(&update)
		assert.Nil(t, err)
		gateway, err := CreateIngressGateway(node.Network, node.MacAddress)
		assert.EqualError(t, err, "macos is unsupported for ingress gateways")
		assert.Equal(t, models.Node{}, gateway)
	})
	t.Run("SuccessfulCreate", func(*testing.T) {
		deleteAllNodes()
		node := createTestNode()
		gateway, err := CreateIngressGateway(node.Network, node.MacAddress)
		assert.Nil(t, err)
		assert.Equal(t, "yes", gateway.PullChanges)
		assert.Equal(t, "no", gateway.UDPHolePunch)
		assert.Contains(t, gateway.PostUp, "iptables -A FORWARD")
		assert.Contains(t, gateway.PostDown, "iptables -D FORWARD")

	})
}

func TestDeleteIngressGateway(t *testing.T) {
	database.InitializeDatabase()
	deleteAllNetworks()
	createNet()
	t.Run("NoGateway", func(*testing.T) {
		node, err := DeleteIngressGateway("skynet", "01:02:03:04:05:06")
		assert.EqualError(t, err, "could not find any records")
		assert.Equal(t, models.Node{}, node)
	})
	t.Run("NoExtClient", func(*testing.T) {
		node := createTestNode()
		gateway, err := CreateIngressGateway(node.Network, node.MacAddress)
		assert.Nil(t, err)
		node, err = DeleteIngressGateway(gateway.Network, gateway.MacAddress)
		assert.Nil(t, err)
		assert.Equal(t, "no", node.IsIngressGateway)
	})
}

func TestNodeUpdate(t *testing.T) {
	database.InitializeDatabase()
	deleteAllNetworks()
	createNet()
	node := createTestNode()
	t.Run("NewMac", func(*testing.T) {
		update, err := GetNode("01:02:03:04:05:06", "skynet")
		assert.Nil(t, err)
		update.MacAddress = "01:02:03:04:05:07"
		err = node.Update(&update)
		assert.EqualError(t, err, "failed to update node 01:02:03:04:05:07, cannot change macaddress.")
	})
	t.Run("GoodUpdate", func(*testing.T) {
		update, err := GetNode("01:02:03:04:05:06", "skynet")
		assert.Nil(t, err)
		update.ListenPort = 51820
		err = node.Update(&update)
		assert.Nil(t, err)
	})
}

//func TestDeleteNode(t *testing.T) {
//	database.InitializeDatabase()
//	deleteAllNetworks()
//	createNet()
//	createTestNode()
//	t.Run("BadMacWithExterminate", func(*testing.T) {
//		err := DeleteNode("01:02:03:04:05:07###skynet", true)
//		t.Log(err)
//	})
//}

//
////func TestUpdateNode(t *testing.T) {
////}
func deleteAllNodes() {
	nodes, _ := logic.GetAllNodes()
	for _, node := range nodes {
		key := node.MacAddress + "###" + node.Network
		DeleteNode(key, true)
	}
}
