package orchestrator

import (
	"context"
	"net"

	"github.com/gravitl/netmaker/db"
	core "github.com/gravitl/netmaker/orchestrator"
	"github.com/gravitl/netmaker/pro/orchestrator/extensions"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"github.com/stretchr/testify/require"

	testutils "github.com/gravitl/netmaker/test/utils"
	"github.com/stretchr/testify/suite"
	"gorm.io/datatypes"
)

type ProNodeOrchestratorTestSuite struct {
	ctx context.Context
	suite.Suite
	db string
}

func NewSqliteProNodeOrchestratorTestSuite() *ProNodeOrchestratorTestSuite {
	return &ProNodeOrchestratorTestSuite{
		db: "sqlite",
	}
}

func NewPostgresProNodeOrchestratorTestSuite() *ProNodeOrchestratorTestSuite {
	return &ProNodeOrchestratorTestSuite{
		db: "postgres",
	}
}

func (c *ProNodeOrchestratorTestSuite) SetupSuite() {
	switch c.db {
	case "postgres":
		testutils.InitPostgres(c.T())
	default:
		testutils.InitSqlite(c.T())
	}

	core.InitializeRepository(extensions.NewProFactory())
	testutils.CreateDefaultOrgAndTenant(c.T(), db.WithContext(c.T().Context()))

	defaultTenant := &schema.Tenant{}
	err := defaultTenant.GetDefault(db.WithContext(c.T().Context()))
	require.NoError(c.T(), err)

	c.T().Context()

	c.ctx = scope.WithContext(db.WithContext(c.T().Context()), scope.TenantScope, defaultTenant.ID)
}

func (c *ProNodeOrchestratorTestSuite) TearDownSuite() {
	switch c.db {
	case "postgres":
		testutils.CleanupPostgres(c.T())
	default:
		testutils.CleanupSqlite(c.T())
	}
}

func (c *ProNodeOrchestratorTestSuite) TestCreateNode() {
	host := testutils.CreateHost(c.T(), c.ctx, "host-0")
	networkIPv4 := testutils.CreateIPv4Network(c.T(), c.ctx, "network-ipv4")
	networkIPv6 := testutils.CreateIPv6Network(c.T(), c.ctx, "network-ipv6")
	networkIPv10 := testutils.CreateIPv10Network(c.T(), c.ctx, "network-ipv10")

	c.Run("IPv4 Network", func() {
		node, err := core.GetRepository().NodeOrchestrator().CreateNode(c.ctx, host, networkIPv4)
		c.Require().NoError(err)
		c.Require().Equal(host.ID.String(), node.HostID)
		c.Require().NotNil(node.Host)
		c.Require().Equal(networkIPv4.ID, node.NetworkID)
		c.Require().NotNil(node.Network)
		c.Require().True(node.Connected)
		c.Require().NotEmpty(node.Address)
		_, _, err = net.ParseCIDR(node.Address)
		c.Require().NoError(err)
		c.Require().Empty(node.Address6)
		c.Require().Contains(host.Nodes, node.ID)

		testutils.DeleteNode(c.T(), c.ctx, node)
	})

	c.Run("IPv6 Network", func() {
		node, err := core.GetRepository().NodeOrchestrator().CreateNode(c.ctx, host, networkIPv6)
		c.Require().NoError(err)
		c.Require().Equal(host.ID.String(), node.HostID)
		c.Require().NotNil(node.Host)
		c.Require().Equal(networkIPv6.ID, node.NetworkID)
		c.Require().NotNil(node.Network)
		c.Require().True(node.Connected)
		c.Require().Empty(node.Address)
		c.Require().NotEmpty(node.Address6)
		_, _, err = net.ParseCIDR(node.Address6)
		c.Require().NoError(err)
		c.Require().Contains(host.Nodes, node.ID)

		testutils.DeleteNode(c.T(), c.ctx, node)
	})

	c.Run("IPv10 Network", func() {
		node, err := core.GetRepository().NodeOrchestrator().CreateNode(c.ctx, host, networkIPv10)
		c.Require().NoError(err)
		c.Require().Equal(node.HostID, host.ID.String())
		c.Require().NotNil(node.Host)
		c.Require().Equal(networkIPv10.ID, node.NetworkID)
		c.Require().NotNil(node.Network)
		c.Require().True(node.Connected)
		c.Require().NotEmpty(node.Address)
		_, _, err = net.ParseCIDR(node.Address)
		c.Require().NoError(err)
		c.Require().NotEmpty(node.Address6)
		_, _, err = net.ParseCIDR(node.Address6)
		c.Require().NoError(err)
		c.Require().Contains(host.Nodes, node.ID)

		testutils.DeleteNode(c.T(), c.ctx, node)
	})

	testutils.DeleteNetwork(c.T(), c.ctx, networkIPv4)
	testutils.DeleteNetwork(c.T(), c.ctx, networkIPv6)
	testutils.DeleteNetwork(c.T(), c.ctx, networkIPv10)
	testutils.DeleteHost(c.T(), c.ctx, host)
}

func (c *ProNodeOrchestratorTestSuite) TestCreateNodeWithDefaultHost() {
	network := testutils.CreateIPv10Network(c.T(), c.ctx, "network-0")

	c.Run("Linux", func() {
		host := testutils.CreateHost(c.T(), c.ctx, "host-0")

		host.OS = "linux"
		host.IsDefault = true

		err := host.Upsert(c.ctx)
		c.Require().NoError(err)

		node, err := core.GetRepository().NodeOrchestrator().CreateNode(c.ctx, host, network)
		c.Require().NoError(err)
		c.Require().True(node.IsGateway)
		c.Require().False(node.IsInternetGateway)
		c.Require().Equal("yes", node.IsAutoRelay)
		c.Require().Empty(node.RelayedClients)
		c.Require().Empty(node.RelayedIGWClients)
		c.Require().Equal(datatypes.NewJSONType(map[string]string{}), node.AutoRelayedPeers)

		testutils.DeleteNode(c.T(), c.ctx, node)
		testutils.DeleteHost(c.T(), c.ctx, host)
	})

	c.Run("Windows", func() {
		host := testutils.CreateHost(c.T(), c.ctx, "host-0")

		host.OS = "windows"
		host.IsDefault = true

		err := host.Upsert(c.ctx)
		c.Require().NoError(err)

		_, err = core.GetRepository().NodeOrchestrator().CreateNode(c.ctx, host, network)
		c.Require().ErrorContains(err, "gateway can only be created on linux based node")

		testutils.DeleteHost(c.T(), c.ctx, host)
	})

	c.Run("Darwin", func() {
		host := testutils.CreateHost(c.T(), c.ctx, "host-0")

		host.OS = "darwin"
		host.IsDefault = true

		err := host.Upsert(c.ctx)
		c.Require().NoError(err)

		_, err = core.GetRepository().NodeOrchestrator().CreateNode(c.ctx, host, network)
		c.Require().ErrorContains(err, "gateway can only be created on linux based node")

		testutils.DeleteHost(c.T(), c.ctx, host)
	})

	testutils.DeleteNetwork(c.T(), c.ctx, network)
}

func (c *ProNodeOrchestratorTestSuite) TestCreateNodeWithEnrollmentKey() {
	host := testutils.CreateHost(c.T(), c.ctx, "host-0")
	network := testutils.CreateIPv10Network(c.T(), c.ctx, "network-0")
	tag := testutils.CreateTag(c.T(), c.ctx, "tag-0", network.Name)

	c.Run("With AutoAssignGateway", func() {
		key := &schema.EnrollmentKey{
			AutoAssignGateway: true,
		}

		node, err := core.GetRepository().NodeOrchestrator().CreateNode(c.ctx, host, network, core.UseKey(key))
		c.Require().NoError(err)
		c.Require().True(node.AutoAssignGateway)

		testutils.DeleteNode(c.T(), c.ctx, node)
	})

	c.Run("Without AutoAssignGateway", func() {
		key := &schema.EnrollmentKey{
			AutoAssignGateway: false,
		}

		node, err := core.GetRepository().NodeOrchestrator().CreateNode(c.ctx, host, network, core.UseKey(key))
		c.Require().NoError(err)
		c.Require().False(node.AutoAssignGateway)

		testutils.DeleteNode(c.T(), c.ctx, node)
	})

	c.Run("With Tags", func() {
		key := &schema.EnrollmentKey{
			Tags: []string{tag.ID.String()},
		}

		node, err := core.GetRepository().NodeOrchestrator().CreateNode(c.ctx, host, network, core.UseKey(key))
		c.Require().NoError(err)
		c.Require().Contains(node.Tags, key.Tags[0])

		testutils.DeleteNode(c.T(), c.ctx, node)
	})

	c.Run("Without Tags", func() {
		key := &schema.EnrollmentKey{
			Tags: []string{},
		}

		node, err := core.GetRepository().NodeOrchestrator().CreateNode(c.ctx, host, network, core.UseKey(key))
		c.Require().NoError(err)
		c.Require().Empty(node.Tags)

		testutils.DeleteNode(c.T(), c.ctx, node)
	})

	c.Run("With Gateway", func() {
		gatewayHost := testutils.CreateHost(c.T(), c.ctx, "gateway-0")

		gatewayHost.OS = "linux"
		gatewayHost.IsDefault = true

		gateway, err := core.GetRepository().NodeOrchestrator().CreateNode(c.ctx, gatewayHost, network)
		c.Require().NoError(err)

		key := &schema.EnrollmentKey{
			GatewayID: &gateway.ID,
		}

		node, err := core.GetRepository().NodeOrchestrator().CreateNode(c.ctx, host, network, core.UseKey(key))
		c.Require().NoError(err)
		c.Require().NotNil(node.RelayedByNodeID)
		c.Require().Equal(gateway.ID, *node.RelayedByNodeID)

		err = gateway.Get(c.ctx)
		c.Require().NoError(err)
		c.Require().Contains(gateway.RelayedClients, node.ID)

		testutils.DeleteNode(c.T(), c.ctx, node)
		testutils.DeleteNode(c.T(), c.ctx, gateway)
		testutils.DeleteHost(c.T(), c.ctx, gatewayHost)
	})

	c.Run("Without Gateway", func() {
		key := &schema.EnrollmentKey{}

		node, err := core.GetRepository().NodeOrchestrator().CreateNode(c.ctx, host, network, core.UseKey(key))
		c.Require().NoError(err)

		testutils.DeleteNode(c.T(), c.ctx, node)
	})

	testutils.DeleteTag(c.T(), c.ctx, tag)
	testutils.DeleteNetwork(c.T(), c.ctx, network)
	testutils.DeleteHost(c.T(), c.ctx, host)
}
