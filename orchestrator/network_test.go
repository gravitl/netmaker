package orchestrator

import (
	"fmt"
	"net"
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

func (c *CENodeOrchestratorTestSuite) TestAllocateIPv4() {
	// usable addresses: 10.10.0.1 - 10.10.0.6.
	network := c.createNetwork("network-alloc-ipv4", "10.10.0.0/29", "")
	defer testutils.DeleteNetwork(c.T(), c.ctx, network)
	orch := GetRepository().NetworkOrchestrator()

	allocate := func(node bool) net.IP {
		var ip net.IP
		var err error
		if node {
			ip, err = orch.AllocateNodeIP(c.ctx, network)
		} else {
			ip, err = orch.AllocateExtclientIP(c.ctx, network)
		}
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
		_, err := orch.AllocateNodeIP(c.ctx, network)
		c.Require().ErrorIs(err, ErrIPPoolExhausted)
		_, err = orch.AllocateExtclientIP(c.ctx, network)
		c.Require().ErrorIs(err, ErrIPPoolExhausted)
	})

	c.Run("Reuse Orphaned", func() {
		c.Require().NoError(orch.ReleaseIP(c.ctx, network, "10.10.0.2"))
		c.Require().NoError(orch.ReleaseIP(c.ctx, network, "10.10.0.5/29"))

		c.Require().Equal("10.10.0.5", allocate(false).String())
		c.Require().Equal("10.10.0.2", allocate(true).String())
		_, err := orch.AllocateNodeIP(c.ctx, network)
		c.Require().ErrorIs(err, ErrIPPoolExhausted)
	})
}

func (c *CENodeOrchestratorTestSuite) TestAllocateOrphanedIPFirst() {
	network := c.createNetwork("network-alloc-orphaned", "10.11.0.0/24", "")
	defer testutils.DeleteNetwork(c.T(), c.ctx, network)
	orch := GetRepository().NetworkOrchestrator()

	allocate := func(node bool, expected string) {
		var ip net.IP
		var err error
		if node {
			ip, err = orch.AllocateNodeIP(c.ctx, network)
		} else {
			ip, err = orch.AllocateExtclientIP(c.ctx, network)
		}
		c.Require().NoError(err)
		c.Require().Equal(expected, ip.String())
	}

	for i := 1; i <= 10; i++ {
		allocate(true, fmt.Sprintf("10.11.0.%d", i))
	}
	allocate(false, "10.11.0.254")
	allocate(false, "10.11.0.253")
	for _, address := range []string{"10.11.0.9", "10.11.0.10", "10.11.0.2", "10.11.0.253", "10.11.0.254"} {
		c.Require().NoError(orch.ReleaseIP(c.ctx, network, address))
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

func (c *CENodeOrchestratorTestSuite) TestAllocateIPv6() {
	network := c.createNetwork("network-alloc-ipv6", "", "fd00:10::/64")
	defer testutils.DeleteNetwork(c.T(), c.ctx, network)
	orch := GetRepository().NetworkOrchestrator()

	ip, err := orch.AllocateNodeIPv6(c.ctx, network)
	c.Require().NoError(err)
	c.Require().Equal("fd00:10::1", ip.String())

	ip, err = orch.AllocateExtclientIPv6(c.ctx, network)
	c.Require().NoError(err)
	c.Require().Equal("fd00:10::ffff:ffff:ffff:fffe", ip.String())

	_, err = orch.AllocateNodeIP(c.ctx, network)
	c.Require().Error(err)
}

func (c *CENodeOrchestratorTestSuite) TestClaimIP() {
	network := c.createNetwork("network-claim", "10.20.0.0/24", "")
	defer testutils.DeleteNetwork(c.T(), c.ctx, network)
	orch := GetRepository().NetworkOrchestrator()

	c.Run("Skipped By Cursor", func() {
		c.Require().NoError(orch.ClaimIP(c.ctx, network, "10.20.0.1/24", schema.IPOwnerNode))
		c.Require().NoError(orch.ClaimIP(c.ctx, network, "10.20.0.2", schema.IPOwnerNode))

		ip, err := orch.AllocateNodeIP(c.ctx, network)
		c.Require().NoError(err)
		c.Require().Equal("10.20.0.3", ip.String())
	})

	c.Run("Already Allocated", func() {
		err := orch.ClaimIP(c.ctx, network, "10.20.0.3", schema.IPOwnerNode)
		c.Require().ErrorIs(err, ErrIPAlreadyAllocated)
	})

	c.Run("Orphaned", func() {
		c.Require().NoError(orch.ReleaseIP(c.ctx, network, "10.20.0.3"))
		c.Require().NoError(orch.ClaimIP(c.ctx, network, "10.20.0.3", schema.IPOwnerNode))
	})

	c.Run("Out Of Range", func() {
		c.Require().Error(orch.ClaimIP(c.ctx, network, "10.20.1.1", schema.IPOwnerNode))
		c.Require().Error(orch.ClaimIP(c.ctx, network, "10.20.0.0", schema.IPOwnerNode))
		c.Require().Error(orch.ClaimIP(c.ctx, network, "10.20.0.255", schema.IPOwnerNode))
	})
}

func (c *CENodeOrchestratorTestSuite) TestAllocateIPWithoutPool() {
	network := c.createNetwork("network-no-pool", "10.30.0.0/24", "")
	defer testutils.DeleteNetwork(c.T(), c.ctx, network)
	c.Require().NoError(db.FromContext(c.ctx).Where("network_id = ?", network.ID).Delete(&schema.IPPool{}).Error)

	_, err := GetRepository().NetworkOrchestrator().AllocateNodeIP(c.ctx, network)
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
				ip, err = orch.AllocateNodeIP(c.ctx, network)
			} else {
				ip, err = orch.AllocateExtclientIP(c.ctx, network)
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
