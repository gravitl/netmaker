package orchestrator

import (
	"context"
	"net"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/orchestrator/extensions"
	testutils "github.com/gravitl/netmaker/test/utils"
	"github.com/stretchr/testify/suite"
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
		node, err := GetRepository().NodeOrchestrator().CreateNode(db.WithContext(context.TODO()), host, networkIPv4, SkipHostUpdate(), SkipPublishPeerUpdate())
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
		node, err := GetRepository().NodeOrchestrator().CreateNode(db.WithContext(context.TODO()), host, networkIPv6, SkipHostUpdate(), SkipPublishPeerUpdate())
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
		node, err := GetRepository().NodeOrchestrator().CreateNode(db.WithContext(context.TODO()), host, networkIPv10, SkipHostUpdate(), SkipPublishPeerUpdate())
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
