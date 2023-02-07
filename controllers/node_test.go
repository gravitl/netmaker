package controller

import (
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/logic/acls"
	"github.com/gravitl/netmaker/logic/acls/nodeacls"
	"github.com/gravitl/netmaker/models"
	"github.com/stretchr/testify/assert"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var nonLinuxHost models.Host
var linuxHost models.Host

func TestCreateEgressGateway(t *testing.T) {
	var gateway models.EgressGatewayRequest
	gateway.Ranges = []string{"10.100.100.0/24"}
	gateway.NetID = "skynet"
	database.InitializeDatabase()
	deleteAllNetworks()
	createNet()
	t.Run("NoNodes", func(t *testing.T) {
		node, err := logic.CreateEgressGateway(gateway)
		assert.Equal(t, models.Node{}, node)
		assert.EqualError(t, err, "could not find any records")
	})
	t.Run("Non-linux node", func(t *testing.T) {
		createnode := createNodeWithParams("", "")
		createNodeHosts()
		createnode.HostID = nonLinuxHost.ID
		err := logic.AssociateNodeToHost(createnode, &nonLinuxHost)
		assert.Nil(t, err)
		gateway.NodeID = createnode.ID.String()
		node, err := logic.CreateEgressGateway(gateway)
		assert.Equal(t, models.Node{}, node)
		assert.EqualError(t, err, "windows is unsupported for egress gateways")
	})
	t.Run("Success-Nat-Enabled", func(t *testing.T) {
		deleteAllNodes()
		testnode := createTestNode()
		gateway.NodeID = testnode.ID.String()
		gateway.NatEnabled = "yes"

		node, err := logic.CreateEgressGateway(gateway)
		t.Log(node.EgressGatewayNatEnabled)
		assert.Nil(t, err)
	})
	t.Run("Success-Nat-Disabled", func(t *testing.T) {
		deleteAllNodes()
		testnode := createTestNode()
		gateway.NodeID = testnode.ID.String()
		gateway.NatEnabled = "no"

		node, err := logic.CreateEgressGateway(gateway)
		t.Log(node.EgressGatewayNatEnabled)
		assert.Nil(t, err)
	})
	t.Run("Success", func(t *testing.T) {
		var gateway models.EgressGatewayRequest
		gateway.Ranges = []string{"10.100.100.0/24"}
		gateway.NetID = "skynet"
		deleteAllNodes()
		testnode := createTestNode()
		gateway.NodeID = testnode.ID.String()

		node, err := logic.CreateEgressGateway(gateway)
		t.Log(node)
		assert.Nil(t, err)
		assert.Equal(t, true, node.IsEgressGateway)
		assert.Equal(t, gateway.Ranges, node.EgressGatewayRanges)
	})

}
func TestDeleteEgressGateway(t *testing.T) {
	var gateway models.EgressGatewayRequest
	database.InitializeDatabase()
	deleteAllNetworks()
	createNet()
	testnode := createTestNode()
	gateway.Ranges = []string{"10.100.100.0/24"}
	gateway.NetID = "skynet"
	gateway.NodeID = testnode.ID.String()
	t.Run("Success", func(t *testing.T) {
		node, err := logic.CreateEgressGateway(gateway)
		assert.Nil(t, err)
		assert.Equal(t, true, node.IsEgressGateway)
		assert.Equal(t, []string{"10.100.100.0/24"}, node.EgressGatewayRanges)
		node, err = logic.DeleteEgressGateway(gateway.NetID, gateway.NodeID)
		assert.Nil(t, err)
		assert.Equal(t, false, node.IsEgressGateway)
		assert.Equal(t, []string([]string{}), node.EgressGatewayRanges)
	})
	t.Run("NotGateway", func(t *testing.T) {
		node, err := logic.DeleteEgressGateway(gateway.NetID, gateway.NodeID)
		assert.Nil(t, err)
		assert.Equal(t, false, node.IsEgressGateway)
		assert.Equal(t, []string([]string{}), node.EgressGatewayRanges)
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
		assert.Nil(t, node)
	})
	t.Run("NoNodes", func(t *testing.T) {
		node, err := logic.GetNetworkNodes("skynet")
		assert.Nil(t, err)
		assert.Nil(t, node)
	})
	t.Run("Success", func(t *testing.T) {
		createTestNode()
		node, err := logic.GetNetworkNodes("skynet")
		assert.Nil(t, err)
		assert.NotEqual(t, []models.LegacyNode(nil), node)
	})

}

func TestValidateEgressGateway(t *testing.T) {
	var gateway models.EgressGatewayRequest
	t.Run("EmptyRange", func(t *testing.T) {
		gateway.Ranges = []string{}
		err := logic.ValidateEgressGateway(gateway)
		assert.EqualError(t, err, "IP Ranges Cannot Be Empty")
	})
	t.Run("Success", func(t *testing.T) {
		gateway.Ranges = []string{"10.100.100.0/24"}
		err := logic.ValidateEgressGateway(gateway)
		assert.Nil(t, err)
	})
}

func TestNodeACLs(t *testing.T) {
	deleteAllNodes()
	node1 := createNodeWithParams("", "10.0.0.50/32")
	node2 := createNodeWithParams("", "10.0.0.100/32")
	logic.AssociateNodeToHost(node1, &linuxHost)
	logic.AssociateNodeToHost(node2, &linuxHost)
	t.Run("acls not present", func(t *testing.T) {
		currentACL, err := nodeacls.FetchAllACLs(nodeacls.NetworkID(node1.Network))
		assert.Nil(t, err)
		assert.NotNil(t, currentACL)
		node1ACL, err := nodeacls.FetchNodeACL(nodeacls.NetworkID(node1.Network), nodeacls.NodeID(node1.ID.String()))
		assert.Nil(t, err)
		assert.NotNil(t, node1ACL)
		assert.Equal(t, acls.Allowed, node1ACL[acls.AclID(node2.ID.String())])
	})
	t.Run("node acls exists after creates", func(t *testing.T) {
		node1ACL, err := nodeacls.FetchNodeACL(nodeacls.NetworkID(node1.Network), nodeacls.NodeID(node1.ID.String()))
		assert.Nil(t, err)
		assert.NotNil(t, node1ACL)
		node2ACL, err := nodeacls.FetchNodeACL(nodeacls.NetworkID(node2.Network), nodeacls.NodeID(node2.ID.String()))
		assert.Nil(t, err)
		assert.NotNil(t, node2ACL)
		assert.Equal(t, acls.Allowed, node2ACL[acls.AclID(node1.ID.String())])
	})
	t.Run("node acls correct after fetch", func(t *testing.T) {
		node1ACL, err := nodeacls.FetchNodeACL(nodeacls.NetworkID(node1.Network), nodeacls.NodeID(node1.ID.String()))
		assert.Nil(t, err)
		assert.Equal(t, acls.Allowed, node1ACL[acls.AclID(node2.ID.String())])
	})
	t.Run("node acls correct after modify", func(t *testing.T) {
		node1ACL, err := nodeacls.FetchNodeACL(nodeacls.NetworkID(node1.Network), nodeacls.NodeID(node1.ID.String()))
		assert.Nil(t, err)
		assert.NotNil(t, node1ACL)
		node2ACL, err := nodeacls.FetchNodeACL(nodeacls.NetworkID(node2.Network), nodeacls.NodeID(node2.ID.String()))
		assert.Nil(t, err)
		assert.NotNil(t, node2ACL)
		currentACL, err := nodeacls.DisallowNodes(nodeacls.NetworkID(node1.Network), nodeacls.NodeID(node1.ID.String()), nodeacls.NodeID(node2.ID.String()))
		assert.Nil(t, err)
		assert.Equal(t, acls.NotAllowed, currentACL[acls.AclID(node1.ID.String())][acls.AclID(node2.ID.String())])
		assert.Equal(t, acls.NotAllowed, currentACL[acls.AclID(node2.ID.String())][acls.AclID(node1.ID.String())])
		currentACL.Save(acls.ContainerID(node1.Network))
	})
	t.Run("node acls correct after add new node not allowed", func(t *testing.T) {
		node3 := createNodeWithParams("", "10.0.0.100/32")
		createNodeHosts()
		n, e := logic.GetNetwork(node3.Network)
		assert.Nil(t, e)
		n.DefaultACL = "no"
		e = logic.SaveNetwork(&n)
		assert.Nil(t, e)
		err := logic.AssociateNodeToHost(node3, &linuxHost)
		assert.Nil(t, err)
		currentACL, err := nodeacls.FetchAllACLs(nodeacls.NetworkID(node3.Network))
		assert.Nil(t, err)
		assert.NotNil(t, currentACL)
		assert.Equal(t, acls.NotAllowed, currentACL[acls.AclID(node1.ID.String())][acls.AclID(node3.ID.String())])
		nodeACL, err := nodeacls.CreateNodeACL(nodeacls.NetworkID(node3.Network), nodeacls.NodeID(node3.ID.String()), acls.NotAllowed)
		assert.Nil(t, err)
		nodeACL.Save(acls.ContainerID(node3.Network), acls.AclID(node3.ID.String()))
		currentACL, err = nodeacls.FetchAllACLs(nodeacls.NetworkID(node3.Network))
		assert.Nil(t, err)
		assert.Equal(t, acls.NotAllowed, currentACL[acls.AclID(node1.ID.String())][acls.AclID(node3.ID.String())])
		assert.Equal(t, acls.NotAllowed, currentACL[acls.AclID(node2.ID.String())][acls.AclID(node3.ID.String())])
	})
	t.Run("node acls removed", func(t *testing.T) {
		retNetworkACL, err := nodeacls.RemoveNodeACL(nodeacls.NetworkID(node1.Network), nodeacls.NodeID(node1.ID.String()))
		assert.Nil(t, err)
		assert.NotNil(t, retNetworkACL)
		assert.Equal(t, acls.NotPresent, retNetworkACL[acls.AclID(node2.ID.String())][acls.AclID(node1.ID.String())])
	})
	deleteAllNodes()
}

func deleteAllNodes() {
	database.DeleteAllRecords(database.NODES_TABLE_NAME)
}

func createTestNode() *models.Node {
	createNodeHosts()
	n := createNodeWithParams("skynet", "")
	_ = logic.AssociateNodeToHost(n, &linuxHost)
	return n
}

func createNodeWithParams(network, address string) *models.Node {
	_, ipnet, _ := net.ParseCIDR("10.0.0.1/32")
	tmpCNode := models.CommonNode{
		ID:      uuid.New(),
		Network: "skynet",
		Address: *ipnet,
		DNSOn:   true,
	}
	if len(network) > 0 {
		tmpCNode.Network = network
	}
	if len(address) > 0 {
		_, ipnet2, _ := net.ParseCIDR(address)
		tmpCNode.Address = *ipnet2
	}
	createnode := models.Node{
		CommonNode: tmpCNode,
	}
	return &createnode
}

func createNodeHosts() {
	k, _ := wgtypes.ParseKey("DM5qhLAE20PG9BbfBCger+Ac9D2NDOwCtY1rbYDLf34=")
	linuxHost = models.Host{
		ID:        uuid.New(),
		PublicKey: k.PublicKey(),
		HostPass:  "password",
		OS:        "linux",
		Name:      "linuxhost",
	}
	_ = logic.CreateHost(&linuxHost)
	nonLinuxHost = models.Host{
		ID:        uuid.New(),
		OS:        "windows",
		PublicKey: k.PublicKey(),
		Name:      "windowshost",
		HostPass:  "password",
	}

	_ = logic.CreateHost(&nonLinuxHost)
}
