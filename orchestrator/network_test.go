package orchestrator

import (
	"fmt"
	"net"
	"net/netip"
	"sync"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	testutils "github.com/gravitl/netmaker/test/utils"
)

func (c *CENodeOrchestratorTestSuite) createNetwork(name, addressRange, addressRange6 string) *schema.Network {
	network := &schema.Network{
		ID:            uuid.NewString(),
		TenantID:      scope.ID(c.ctx),
		Name:          name,
		AddressRange:  addressRange,
		AddressRange6: addressRange6,
	}
	c.Require().NoError(network.Create(c.ctx))
	c.Require().NoError(network.CreateIPPools(c.ctx))
	return network
}

// allocateIP allocates an IPv4 address on the network to a new node (or
// extclient), and records its owner in owners.
func (c *CENodeOrchestratorTestSuite) allocateIP(network *schema.Network, node bool, owners map[string]IPOwner) (net.IP, error) {
	orch := GetRepository().NetworkOrchestrator()
	owner := IPOwner{Type: schema.IPOwnerExtClient, ID: uuid.NewString()}
	if node {
		owner.Type = schema.IPOwnerNode
	}

	var ip net.IP
	var err error
	if node {
		ip, err = orch.AllocateNodeIP(c.ctx, network, owner.ID)
	} else {
		ip, err = orch.AllocateExtclientIP(c.ctx, network, owner.ID)
	}
	if err == nil {
		owners[ip.String()] = owner
	}
	return ip, err
}

// releaseIP releases the address (plain or in CIDR notation) on the network
// as its owner recorded in owners.
func (c *CENodeOrchestratorTestSuite) releaseIP(network *schema.Network, address string, owners map[string]IPOwner) error {
	ip := address
	if prefix, err := netip.ParsePrefix(address); err == nil {
		ip = prefix.Addr().String()
	}
	owner := owners[ip]
	return GetRepository().NetworkOrchestrator().ReleaseIP(c.ctx, network, address, owner.Type, owner.ID)
}

func (c *CENodeOrchestratorTestSuite) TestAllocateIPv4() {
	// usable addresses: 10.10.0.1 - 10.10.0.6.
	network := c.createNetwork("network-alloc-ipv4", "10.10.0.0/29", "")
	defer testutils.DeleteNetwork(c.T(), c.ctx, network)
	owners := make(map[string]IPOwner)

	allocate := func(node bool) net.IP {
		ip, err := c.allocateIP(network, node, owners)
		c.Require().NoError(err)
		return ip
	}

	c.Run("Cursors", func() {
		c.Require().Equal("10.10.0.1", allocate(true).String())
		c.Require().Equal("10.10.0.2", allocate(true).String())
		c.Require().Equal("10.10.0.6", allocate(false).String())
		c.Require().Equal("10.10.0.5", allocate(false).String())
		c.Require().Equal("10.10.0.3", allocate(true).String())
		c.Require().Equal("10.10.0.4", allocate(false).String())
	})

	c.Run("Exhausted", func() {
		_, err := c.allocateIP(network, true, owners)
		c.Require().ErrorIs(err, ErrIPPoolExhausted)
		_, err = c.allocateIP(network, false, owners)
		c.Require().ErrorIs(err, ErrIPPoolExhausted)
	})

	c.Run("Reuse Orphaned", func() {
		c.Require().NoError(c.releaseIP(network, "10.10.0.2", owners))
		c.Require().NoError(c.releaseIP(network, "10.10.0.5/29", owners))

		c.Require().Equal("10.10.0.5", allocate(false).String())
		c.Require().Equal("10.10.0.2", allocate(true).String())
		_, err := c.allocateIP(network, true, owners)
		c.Require().ErrorIs(err, ErrIPPoolExhausted)
	})
}

func (c *CENodeOrchestratorTestSuite) TestAllocateOrphanedIPFirst() {
	network := c.createNetwork("network-alloc-orphaned", "10.11.0.0/24", "")
	defer testutils.DeleteNetwork(c.T(), c.ctx, network)
	owners := make(map[string]IPOwner)

	allocate := func(node bool, expected string) {
		ip, err := c.allocateIP(network, node, owners)
		c.Require().NoError(err)
		c.Require().Equal(expected, ip.String())
	}

	for i := 1; i <= 10; i++ {
		allocate(true, fmt.Sprintf("10.11.0.%d", i))
	}
	allocate(false, "10.11.0.254")
	allocate(false, "10.11.0.253")
	for _, address := range []string{"10.11.0.9", "10.11.0.10", "10.11.0.2", "10.11.0.253", "10.11.0.254"} {
		c.Require().NoError(c.releaseIP(network, address, owners))
	}

	// orphaned addresses are reused before the cursors advance: nodes take the
	// one nearest to the start of the range, extclients the one nearest to its
	// end (compared numerically, .9 comes before .10).
	allocate(true, "10.11.0.2")
	allocate(false, "10.11.0.254")
	allocate(true, "10.11.0.9")
	allocate(false, "10.11.0.253")
	allocate(true, "10.11.0.10")

	// with no orphaned addresses left, the cursors resume.
	allocate(true, "10.11.0.11")
	allocate(false, "10.11.0.252")
}

func (c *CENodeOrchestratorTestSuite) TestReleaseIPOwner() {
	network := c.createNetwork("network-release-owner", "10.12.0.0/24", "")
	defer testutils.DeleteNetwork(c.T(), c.ctx, network)
	orch := GetRepository().NetworkOrchestrator()

	ip, err := orch.AllocateNodeIP(c.ctx, network, "node-a")
	c.Require().NoError(err)
	c.Require().Equal("10.12.0.1", ip.String())

	c.Run("Other Owner", func() {
		// neither another node nor an extclient with the same ID can release it.
		c.Require().NoError(orch.ReleaseIP(c.ctx, network, "10.12.0.1", schema.IPOwnerNode, "node-b"))
		c.Require().NoError(orch.ReleaseIP(c.ctx, network, "10.12.0.1", schema.IPOwnerExtClient, "node-a"))

		ip, err := orch.AllocateNodeIP(c.ctx, network, "node-b")
		c.Require().NoError(err)
		c.Require().Equal("10.12.0.2", ip.String())
	})

	c.Run("Stale Release", func() {
		c.Require().NoError(orch.ReleaseIP(c.ctx, network, "10.12.0.1", schema.IPOwnerNode, "node-a"))

		// a released address has no owner.
		allocation := &schema.IPAllocation{TenantID: network.TenantID, NetworkID: network.ID}
		allocation.SetAddress(netip.MustParseAddr("10.12.0.1"))
		c.Require().NoError(allocation.Get(c.ctx))
		c.Require().Equal(schema.IPOrphaned, allocation.State)
		c.Require().Empty(allocation.OwnerType)
		c.Require().Empty(allocation.OwnerID)

		ip, err := orch.AllocateNodeIP(c.ctx, network, "node-c")
		c.Require().NoError(err)
		c.Require().Equal("10.12.0.1", ip.String())

		// releasing again as the previous owner does not release it from the
		// new one.
		c.Require().NoError(orch.ReleaseIP(c.ctx, network, "10.12.0.1", schema.IPOwnerNode, "node-a"))
		ip, err = orch.AllocateNodeIP(c.ctx, network, "node-d")
		c.Require().NoError(err)
		c.Require().Equal("10.12.0.3", ip.String())
	})
}

func (c *CENodeOrchestratorTestSuite) TestAllocateIPv6() {
	network := c.createNetwork("network-alloc-ipv6", "", "fd00:10::/64")
	defer testutils.DeleteNetwork(c.T(), c.ctx, network)
	orch := GetRepository().NetworkOrchestrator()

	ip, err := orch.AllocateNodeIPv6(c.ctx, network, uuid.NewString())
	c.Require().NoError(err)
	c.Require().Equal("fd00:10::1", ip.String())

	ip, err = orch.AllocateExtclientIPv6(c.ctx, network, uuid.NewString())
	c.Require().NoError(err)
	c.Require().Equal("fd00:10::ffff:ffff:ffff:fffe", ip.String())

	_, err = orch.AllocateNodeIP(c.ctx, network, uuid.NewString())
	c.Require().Error(err)
}

func (c *CENodeOrchestratorTestSuite) TestClaimIP() {
	network := c.createNetwork("network-claim", "10.20.0.0/24", "")
	defer testutils.DeleteNetwork(c.T(), c.ctx, network)
	orch := GetRepository().NetworkOrchestrator()

	c.Run("Skipped By Cursor", func() {
		c.Require().NoError(orch.ClaimIP(c.ctx, network, "10.20.0.1/24", schema.IPOwnerNode, "node-a"))
		c.Require().NoError(orch.ClaimIP(c.ctx, network, "10.20.0.2", schema.IPOwnerNode, "node-b"))

		ip, err := orch.AllocateNodeIP(c.ctx, network, "node-c")
		c.Require().NoError(err)
		c.Require().Equal("10.20.0.3", ip.String())
	})

	c.Run("Already Allocated", func() {
		err := orch.ClaimIP(c.ctx, network, "10.20.0.3", schema.IPOwnerNode, "node-d")
		c.Require().ErrorIs(err, ErrIPAlreadyAllocated)
	})

	c.Run("Orphaned", func() {
		c.Require().NoError(orch.ReleaseIP(c.ctx, network, "10.20.0.3", schema.IPOwnerNode, "node-c"))
		c.Require().NoError(orch.ClaimIP(c.ctx, network, "10.20.0.3", schema.IPOwnerNode, "node-d"))
	})

	c.Run("Out Of Range", func() {
		c.Require().Error(orch.ClaimIP(c.ctx, network, "10.20.1.1", schema.IPOwnerNode, "node-e"))
		c.Require().Error(orch.ClaimIP(c.ctx, network, "10.20.0.0", schema.IPOwnerNode, "node-e"))
		c.Require().Error(orch.ClaimIP(c.ctx, network, "10.20.0.255", schema.IPOwnerNode, "node-e"))
	})
}

func (c *CENodeOrchestratorTestSuite) TestAllocateIPWithoutPool() {
	network := c.createNetwork("network-no-pool", "10.30.0.0/24", "")
	defer testutils.DeleteNetwork(c.T(), c.ctx, network)
	c.Require().NoError(db.FromContext(c.ctx).Where("network_id = ?", network.ID).Delete(&schema.IPPool{}).Error)

	_, err := GetRepository().NetworkOrchestrator().AllocateNodeIP(c.ctx, network, uuid.NewString())
	c.Require().ErrorIs(err, ErrIPPoolNotFound)
}

func (c *CENodeOrchestratorTestSuite) TestAllocateIPConcurrently() {
	network := c.createNetwork("network-alloc-concurrent", "10.40.0.0/24", "")
	defer testutils.DeleteNetwork(c.T(), c.ctx, network)
	orch := GetRepository().NetworkOrchestrator()

	const allocations = 40
	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		ips = make(map[string]struct{})
	)
	for i := 0; i < allocations; i++ {
		wg.Add(1)
		go func(node bool) {
			defer wg.Done()
			var ip net.IP
			var err error
			if node {
				ip, err = orch.AllocateNodeIP(c.ctx, network, uuid.NewString())
			} else {
				ip, err = orch.AllocateExtclientIP(c.ctx, network, uuid.NewString())
			}
			c.NoError(err)
			mu.Lock()
			ips[ip.String()] = struct{}{}
			mu.Unlock()
		}(i%2 == 0)
	}
	wg.Wait()

	c.Require().Len(ips, allocations)
}
