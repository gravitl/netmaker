package orchestrator

import (
	"context"
	"net"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/orchestrator/extensions"
	testutils "github.com/gravitl/netmaker/test/utils"
	"github.com/stretchr/testify/suite"
	"gorm.io/datatypes"
)

type CENodeOrchestratorTestSuite struct {
	suite.Suite
	db string
}

func NewSqliteCENodeOrchestratorTestSuite() *CENodeOrchestratorTestSuite {
	return &CENodeOrchestratorTestSuite{
		db: "sqlite",
	}
}

func NewPostgresCENodeOrchestratorTestSuite() *CENodeOrchestratorTestSuite {
	return &CENodeOrchestratorTestSuite{
		db: "postgres",
	}
}

func (c *CENodeOrchestratorTestSuite) SetupSuite() {
	switch c.db {
	case "postgres":
		testutils.InitPostgres(c.T())
	default:
		testutils.InitSqlite(c.T())
	}

	InitializeRepository(extensions.NewCEFactory())
}

func (c *CENodeOrchestratorTestSuite) TearDownSuite() {
	switch c.db {
	case "postgres":
		testutils.CleanupPostgres(c.T())
	default:
		testutils.CleanupSqlite(c.T())
	}
}

func (c *CENodeOrchestratorTestSuite) TestCreateNode() {
	host := testutils.CreateHost(c.T(), "host-0")
	networkIPv4 := testutils.CreateIPv4Network(c.T(), "network-ipv4")
	networkIPv6 := testutils.CreateIPv6Network(c.T(), "network-ipv6")
	networkIPv10 := testutils.CreateIPv10Network(c.T(), "network-ipv10")

	c.Run("IPv4 Network", func() {
		node, err := GetRepository().NodeOrchestrator().CreateNode(db.WithContext(context.TODO()), host, networkIPv4)
		c.Require().NoError(err)
		c.Require().Equal(node.HostID, host.ID.String())
		c.Require().NotNil(node.Host)
		c.Require().Equal(node.NetworkID, networkIPv4.ID)
		c.Require().NotNil(node.Network)
		c.Require().True(node.Connected)
		c.Require().NotEmpty(node.Address)
		_, _, err = net.ParseCIDR(node.Address)
		c.Require().NoError(err)
		c.Require().Empty(node.Address6)
		c.Require().Contains(host.Nodes, node.ID)

		err = node.Delete(db.WithContext(context.TODO()))
		c.Require().NoError(err)
	})

	c.Run("IPv6 Network", func() {
		node, err := GetRepository().NodeOrchestrator().CreateNode(db.WithContext(context.TODO()), host, networkIPv6)
		c.Require().NoError(err)
		c.Require().Equal(node.HostID, host.ID.String())
		c.Require().NotNil(node.Host)
		c.Require().Equal(node.NetworkID, networkIPv6.ID)
		c.Require().NotNil(node.Network)
		c.Require().True(node.Connected)
		c.Require().Empty(node.Address)
		c.Require().NotEmpty(node.Address6)
		_, _, err = net.ParseCIDR(node.Address6)
		c.Require().NoError(err)
		c.Require().Contains(host.Nodes, node.ID)

		err = node.Delete(db.WithContext(context.TODO()))
		c.Require().NoError(err)
	})

	c.Run("IPv10 Network", func() {
		node, err := GetRepository().NodeOrchestrator().CreateNode(db.WithContext(context.TODO()), host, networkIPv10)
		c.Require().NoError(err)
		c.Require().Equal(node.HostID, host.ID.String())
		c.Require().NotNil(node.Host)
		c.Require().Equal(node.NetworkID, networkIPv10.ID)
		c.Require().NotNil(node.Network)
		c.Require().True(node.Connected)
		c.Require().NotEmpty(node.Address)
		_, _, err = net.ParseCIDR(node.Address)
		c.Require().NoError(err)
		c.Require().NotEmpty(node.Address6)
		_, _, err = net.ParseCIDR(node.Address6)
		c.Require().NoError(err)
		c.Require().Contains(host.Nodes, node.ID)

		err = node.Delete(db.WithContext(context.TODO()))
		c.Require().NoError(err)
	})

	err := networkIPv4.Delete(db.WithContext(context.TODO()))
	c.Require().NoError(err)

	err = networkIPv6.Delete(db.WithContext(context.TODO()))
	c.Require().NoError(err)

	err = networkIPv10.Delete(db.WithContext(context.TODO()))
	c.Require().NoError(err)

	err = host.Delete(db.WithContext(context.TODO()))
	c.Require().NoError(err)
}

func (c *CENodeOrchestratorTestSuite) TestCreateNodeWithDefaultHost() {
	network := testutils.CreateIPv10Network(c.T(), "network-0")

	c.Run("Linux", func() {
		host := testutils.CreateHost(c.T(), "host-0")

		host.OS = "linux"
		host.IsDefault = true

		err := host.Upsert(db.WithContext(context.TODO()))
		c.Require().NoError(err)

		node, err := GetRepository().NodeOrchestrator().CreateNode(db.WithContext(context.TODO()), host, network)
		c.Require().NoError(err)
		c.Require().True(node.IsGateway)
		c.Require().False(node.IsInternetGateway)
		c.Require().Equal(node.IsAutoRelay, "no")
		c.Require().Empty(node.RelayedClients)
		c.Require().Empty(node.RelayedIGWClients)
		c.Require().Equal(node.AutoRelayedPeers, datatypes.NewJSONType(map[string]string{}))

		err = node.Delete(db.WithContext(context.TODO()))
		c.Require().NoError(err)

		err = host.Delete(db.WithContext(context.TODO()))
		c.Require().NoError(err)
	})

	c.Run("Windows", func() {
		host := testutils.CreateHost(c.T(), "host-0")

		host.OS = "windows"
		host.IsDefault = true

		err := host.Upsert(db.WithContext(context.TODO()))
		c.Require().NoError(err)

		_, err = GetRepository().NodeOrchestrator().CreateNode(db.WithContext(context.TODO()), host, network)
		c.Require().ErrorContains(err, "gateway can only be created on linux based node")

		err = host.Delete(db.WithContext(context.TODO()))
		c.Require().NoError(err)
	})

	c.Run("Darwin", func() {
		host := testutils.CreateHost(c.T(), "host-0")

		host.OS = "darwin"
		host.IsDefault = true

		err := host.Upsert(db.WithContext(context.TODO()))
		c.Require().NoError(err)

		_, err = GetRepository().NodeOrchestrator().CreateNode(db.WithContext(context.TODO()), host, network)
		c.Require().ErrorContains(err, "gateway can only be created on linux based node")

		err = host.Delete(db.WithContext(context.TODO()))
		c.Require().NoError(err)
	})

	err := network.Delete(db.WithContext(context.TODO()))
	c.Require().NoError(err)
}

func (c *CENodeOrchestratorTestSuite) TestCreateNodeWithEnrollmentKey() {
	host := testutils.CreateHost(c.T(), "host-0")
	network := testutils.CreateIPv10Network(c.T(), "network-0")

	c.Run("With AutoAssignGateway", func() {
		key := &models.EnrollmentKey{
			AutoAssignGateway: true,
		}

		node, err := GetRepository().NodeOrchestrator().CreateNode(db.WithContext(context.TODO()), host, network, UseKey(key))
		c.Require().NoError(err)
		c.Require().False(node.AutoAssignGateway)
	})

	c.Run("Without AutoAssignGateway", func() {
		key := &models.EnrollmentKey{
			AutoAssignGateway: false,
		}

		node, err := GetRepository().NodeOrchestrator().CreateNode(db.WithContext(context.TODO()), host, network, UseKey(key))
		c.Require().NoError(err)
		c.Require().False(node.AutoAssignGateway)
	})

	c.Run("With Tags", func() {
		key := &models.EnrollmentKey{
			Groups: []models.TagID{"tag-0"},
		}

		node, err := GetRepository().NodeOrchestrator().CreateNode(db.WithContext(context.TODO()), host, network, UseKey(key))
		c.Require().NoError(err)
		c.Require().NotContains(node.Tags, string(key.Groups[0]))
	})

	c.Run("Without Tags", func() {
		key := &models.EnrollmentKey{
			Groups: []models.TagID{},
		}

		node, err := GetRepository().NodeOrchestrator().CreateNode(db.WithContext(context.TODO()), host, network, UseKey(key))
		c.Require().NoError(err)
		c.Require().Empty(node.Tags)
	})

	c.Run("With Gateway", func() {
		gatewayHost := testutils.CreateHost(c.T(), "gateway-0")

		gatewayHost.OS = "linux"
		gatewayHost.IsDefault = true

		gateway, err := GetRepository().NodeOrchestrator().CreateNode(db.WithContext(context.TODO()), gatewayHost, network)
		c.Require().NoError(err)

		key := &models.EnrollmentKey{
			Relay: uuid.MustParse(gateway.ID),
		}

		node, err := GetRepository().NodeOrchestrator().CreateNode(db.WithContext(context.TODO()), host, network, UseKey(key))
		c.Require().NoError(err)
		c.Require().NotNil(node.RelayedByNodeID)
		c.Require().Equal(*node.RelayedByNodeID, gateway.ID)

		err = gateway.Get(db.WithContext(context.TODO()))
		c.Require().NoError(err)
		c.Require().Contains(gateway.RelayedClients, node.ID)
	})

	c.Run("Without Gateway", func() {
		key := &models.EnrollmentKey{}

		node, err := GetRepository().NodeOrchestrator().CreateNode(db.WithContext(context.TODO()), host, network, UseKey(key))
		c.Require().NoError(err)

		err = node.Delete(db.WithContext(context.TODO()))
		c.Require().NoError(err)
	})

	err := network.Delete(db.WithContext(context.TODO()))
	c.Require().NoError(err)

	err = host.Delete(db.WithContext(context.TODO()))
	c.Require().NoError(err)
}
