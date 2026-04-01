package controller

import (
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/stretchr/testify/assert"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var nonLinuxHost schema.Host
var linuxHost schema.Host

func TestGetNetworkNodes(t *testing.T) {
	deleteAllNetworks()
	createNet()
	t.Run("BadNet", func(t *testing.T) {
		node, err := logic.GetNetworkNodes("badnet")
		assert.Nil(t, err)
		assert.Equal(t, []models.Node{}, node)
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
		assert.NotEqual(t, []models.LegacyNode(nil), node)
	})

}

func TestValidateEgressGateway(t *testing.T) {
	var gateway models.EgressGatewayRequest

	t.Run("Success", func(t *testing.T) {
		gateway.Ranges = []string{"10.100.100.0/24"}
		err := logic.ValidateEgressGateway(gateway)
		assert.Nil(t, err)
	})
}

func deleteAllNodes() {
	if servercfg.CacheEnabled() {
		logic.ClearNodeCache()
	}
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
	linuxHost = schema.Host{
		ID:        uuid.New(),
		PublicKey: schema.WgKey{Key: k.PublicKey()},
		HostPass:  "password",
		OS:        "linux",
		Name:      "linuxhost",
	}
	_ = logic.CreateHost(&linuxHost)
	nonLinuxHost = schema.Host{
		ID:        uuid.New(),
		OS:        "windows",
		PublicKey: schema.WgKey{Key: k.PublicKey()},
		Name:      "windowshost",
		HostPass:  "password",
	}

	_ = logic.CreateHost(&nonLinuxHost)
}
