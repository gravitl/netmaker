package orchestrator

import (
	"fmt"
	"time"

	"github.com/c-robinson/iplib"
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	testutils "github.com/gravitl/netmaker/test/utils"
)

// TestAllocateExtclientIP_DenseFromCIDREnd seeds many extclients at the high
// end of the CIDR (where reverse allocation starts) and verifies the next
// allocation still returns a unique IP quickly — the old per-candidate full
// list made this O(E²) and crawled past ~200 clients.
func (c *CENodeOrchestratorTestSuite) TestAllocateExtclientIP_DenseFromCIDREnd() {
	network := &schema.Network{
		ID:           "alloc-dense-net-id",
		TenantID:     scope.ID(c.ctx),
		Name:         "alloc-dense-net",
		AddressRange: "10.200.0.0/24",
	}
	err := network.Create(c.ctx)
	c.Require().NoError(err)
	c.Require().NoError(network.CreateIPPools(c.ctx))
	c.T().Cleanup(func() { testutils.DeleteNetwork(c.T(), c.ctx, network) })

	net4 := iplib.Net4FromStr(network.AddressRange)
	addr := net4.LastAddress()
	const seedCount = 200
	used := make(map[string]struct{}, seedCount)

	for i := 0; i < seedCount; i++ {
		ipStr := addr.String()
		used[ipStr] = struct{}{}
		client := models.ExtClient{
			ClientID:         fmt.Sprintf("seed-%d", i),
			Network:          network.Name,
			Address:          ipStr,
			Enabled:          true,
			IngressGatewayID: "gw-seed",
			PublicKey:        "seed-pubkey",
		}
		c.Require().NoError(logic.SaveExtClient(c.ctx, &client))
		prev, err := net4.PreviousIP(addr)
		c.Require().NoError(err)
		addr = prev
	}

	orch := GetRepository().NetworkOrchestrator()
	start := time.Now()
	owner := uuid.NewString()
	allocated, err := orch.AllocateExtclientIP(c.ctx, network, owner)
	elapsed := time.Since(start)
	c.Require().NoError(err)
	c.Require().NotNil(allocated)
	c.Require().NotContains(used, allocated.String())
	c.Require().Less(elapsed, 2*time.Second, "dense allocation took %s", elapsed)

	owner1 := uuid.NewString()
	reserved1, err := orch.AllocateExtclientIP(c.ctx, network, owner1)
	c.Require().NoError(err)
	c.Require().NotEqual(allocated.String(), reserved1.String())

	owner2 := uuid.NewString()
	reserved2, err := orch.AllocateExtclientIP(c.ctx, network, owner2)
	c.Require().NoError(err)
	c.Require().NotEqual(reserved1.String(), reserved2.String())
	c.Require().NotEqual(allocated.String(), reserved2.String())

	c.Require().NoError(orch.ReleaseIP(c.ctx, network, allocated.String(), schema.IPOwnerExtClient, owner))
	c.Require().NoError(orch.ReleaseIP(c.ctx, network, reserved1.String(), schema.IPOwnerExtClient, owner1))
	c.Require().NoError(orch.ReleaseIP(c.ctx, network, reserved2.String(), schema.IPOwnerExtClient, owner2))
}
