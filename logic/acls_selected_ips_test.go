package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func TestNormalizeAndValidateAclEgressIPs(t *testing.T) {
	originalGetEgressByID := getEgressByID
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
	})

	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "eg-1":
			return schema.Egress{ID: egressID, Range: "10.10.0.0/24"}, nil
		default:
			return schema.Egress{}, errors.New("not found")
		}
	}

	acl := models.Acl{
		Dst: []models.AclPolicyTag{
			{ID: models.EgressID, Value: "eg-1"},
			{ID: models.NetmakerIPAclID, Value: "10.10.0.10"},
		},
	}
	if err := NormalizeAndValidateAclEgressIPs(&acl); err != nil {
		t.Fatalf("expected valid selected ip, got error: %v", err)
	}
	if acl.Dst[1].Value != "10.10.0.10/32" {
		t.Fatalf("expected normalized host CIDR, got %s", acl.Dst[1].Value)
	}
}

func TestNormalizeAndValidateAclEgressIPsRejectsOutsideRange(t *testing.T) {
	originalGetEgressByID := getEgressByID
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
	})

	getEgressByID = func(egressID string) (schema.Egress, error) {
		return schema.Egress{ID: egressID, Range: "10.10.0.0/24"}, nil
	}

	acl := models.Acl{
		Dst: []models.AclPolicyTag{
			{ID: models.EgressID, Value: "eg-1"},
			{ID: models.NetmakerIPAclID, Value: "10.20.0.10"},
		},
	}
	if err := NormalizeAndValidateAclEgressIPs(&acl); err == nil {
		t.Fatal("expected error for selected ip outside egress range")
	}
}

func TestNormalizeAndValidateAclEgressIPsRequiresEgressReference(t *testing.T) {
	acl := models.Acl{
		Dst: []models.AclPolicyTag{
			{ID: models.NetmakerIPAclID, Value: "10.10.0.10"},
		},
	}
	if err := NormalizeAndValidateAclEgressIPs(&acl); err == nil {
		t.Fatal("expected error when egress ip is configured without egress destination")
	}
}

func TestGetEgressToEgressPoliciesForNode(t *testing.T) {
	originalGetEgressByID := getEgressByID
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
	})

	targetID := uuid.New()
	otherID := uuid.New()
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
		},
	}

	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{
			{
				ID:      "match-src",
				Enabled: true,
				AllowedDirection: models.TrafficDirectionUni,
				Src:     []models.AclPolicyTag{{ID: models.EgressID, Value: "src-egress"}},
				Dst:     []models.AclPolicyTag{{ID: models.EgressID, Value: "dst-egress"}},
			},
			{
				ID:      "non-egress",
				Enabled: true,
				Src:     []models.AclPolicyTag{{ID: models.NodeTagID, Value: "tag1"}},
				Dst:     []models.AclPolicyTag{{ID: models.NetmakerIPAclID, Value: "10.0.0.1/32"}},
			},
			{
				ID:      "disabled",
				Enabled: false,
				Src:     []models.AclPolicyTag{{ID: models.EgressID, Value: "src-egress"}},
				Dst:     []models.AclPolicyTag{{ID: models.EgressID, Value: "dst-egress"}},
			},
		}
	}
	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "src-egress":
			return schema.Egress{
				ID:     egressID,
				Status: true,
				Nodes: datatypes.JSONMap{
					targetID.String(): true,
				},
			}, nil
		case "dst-egress":
			return schema.Egress{
				ID:     egressID,
				Status: true,
				Nodes: datatypes.JSONMap{
					otherID.String(): true,
				},
			}, nil
		default:
			return schema.Egress{}, errors.New("not found")
		}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return nil, nil
	}

	filtered := getEgressToEgressPoliciesForNode(targetNode)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 matched policy, got %d", len(filtered))
	}
	if filtered[0].ID != "match-src" {
		t.Fatalf("expected matched policy 'match-src', got %s", filtered[0].ID)
	}
}

func TestIsEgressToEgressPolicyForTarget_MatchesDstRoutingNode(t *testing.T) {
	originalGetEgressByID := getEgressByID
	originalGetEgressByNetwork := getEgressByNetwork
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
		getEgressByNetwork = originalGetEgressByNetwork
	})

	targetID := uuid.New()
	otherID := uuid.New()
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
		},
	}
	policy := models.Acl{
		Enabled: true,
		AllowedDirection: models.TrafficDirectionBi,
		Src:     []models.AclPolicyTag{{ID: models.EgressID, Value: "src-egress"}},
		Dst:     []models.AclPolicyTag{{ID: models.EgressID, Value: "dst-egress"}},
	}
	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "src-egress":
			return schema.Egress{
				ID:     egressID,
				Status: true,
				Nodes: datatypes.JSONMap{
					otherID.String(): true,
				},
			}, nil
		case "dst-egress":
			return schema.Egress{
				ID:     egressID,
				Status: true,
				Nodes: datatypes.JSONMap{
					targetID.String(): true,
				},
			}, nil
		default:
			return schema.Egress{}, errors.New("not found")
		}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return nil, nil
	}

	if !isEgressToEgressPolicyForTarget(policy, targetNode) {
		t.Fatal("expected policy to match when target node routes destination egress")
	}
}

func TestIsEgressToEgressPolicyForTarget_UniDirection_MatchesSrcRoutingNode(t *testing.T) {
	originalGetEgressByID := getEgressByID
	originalGetEgressByNetwork := getEgressByNetwork
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
		getEgressByNetwork = originalGetEgressByNetwork
	})

	targetID := uuid.New()
	otherID := uuid.New()
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
		},
	}
	policy := models.Acl{
		Enabled:          true,
		AllowedDirection: models.TrafficDirectionUni,
		Src:              []models.AclPolicyTag{{ID: models.EgressID, Value: "src-egress"}},
		Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "dst-egress"}},
	}
	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "src-egress":
			return schema.Egress{
				ID:     egressID,
				Status: true,
				Nodes: datatypes.JSONMap{
					targetID.String(): true,
				},
			}, nil
		case "dst-egress":
			return schema.Egress{
				ID:     egressID,
				Status: true,
				Nodes: datatypes.JSONMap{
					otherID.String(): true,
				},
			}, nil
		default:
			return schema.Egress{}, errors.New("not found")
		}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return nil, nil
	}

	if !isEgressToEgressPolicyForTarget(policy, targetNode) {
		t.Fatal("expected uni-directional policy to match when target node routes source egress")
	}
}

// TestIsEgressToEgressPolicyForTarget_UniDirection_MatchesDstOnlyRoutingNode
// asserts the dst-egress-only routing node is also "targeted" by a uni policy,
// because the packet still traverses its FORWARD chain on the way into the LAN.
// Pre-fix, this returned false and the dst-side node never got a firewall rule
// for the policy, dropping src-LAN -> dst-LAN traffic at the gateway.
func TestIsEgressToEgressPolicyForTarget_UniDirection_MatchesDstOnlyRoutingNode(t *testing.T) {
	originalGetEgressByID := getEgressByID
	originalGetEgressByNetwork := getEgressByNetwork
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
		getEgressByNetwork = originalGetEgressByNetwork
	})

	targetID := uuid.New()
	otherID := uuid.New()
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
		},
	}
	policy := models.Acl{
		Enabled:          true,
		AllowedDirection: models.TrafficDirectionUni,
		Src:              []models.AclPolicyTag{{ID: models.EgressID, Value: "src-egress"}},
		Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "dst-egress"}},
	}
	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "src-egress":
			return schema.Egress{
				ID:     egressID,
				Status: true,
				Nodes: datatypes.JSONMap{
					otherID.String(): true,
				},
			}, nil
		case "dst-egress":
			return schema.Egress{
				ID:     egressID,
				Status: true,
				Nodes: datatypes.JSONMap{
					targetID.String(): true,
				},
			}, nil
		default:
			return schema.Egress{}, errors.New("not found")
		}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return nil, nil
	}

	if !isEgressToEgressPolicyForTarget(policy, targetNode) {
		t.Fatal("expected uni-directional policy to match when target node routes destination egress (forward leg required there)")
	}
}

func TestGetEgressAclRulesForTargetNode_EmitsRulesWithSiteToSiteKey(t *testing.T) {
	originalGetEgressByID := getEgressByID
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
	})

	targetID := uuid.New()
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
		},
	}
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{
			{
				ID:               "acl-1",
				Enabled:          true,
				AllowedDirection: models.TrafficDirectionBi,
				Proto:            models.ALL,
				Src: []models.AclPolicyTag{
					{ID: models.EgressID, Value: "src-egress"},
					{ID: models.NetmakerIPAclID, Value: "10.10.0.10/32"},
				},
				Dst: []models.AclPolicyTag{
					{ID: models.EgressID, Value: "dst-egress"},
				},
			},
		}
	}
	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "src-egress":
			return schema.Egress{ID: egressID, Status: true, Range: "10.10.0.0/24", Nodes: datatypes.JSONMap{targetID.String(): true}}, nil
		case "dst-egress":
			return schema.Egress{ID: egressID, Status: true, Range: "10.20.0.0/24", Nodes: datatypes.JSONMap{uuid.New().String(): true}}, nil
		default:
			return schema.Egress{}, errors.New("not found")
		}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) { return nil, nil }

	rules := getEgressAclRulesForTargetNode(targetNode)
	rule, ok := rules["acl-1#xs0"]
	if !ok {
		t.Fatalf("expected site-to-site rule keyed by acl.ID + \"#xs0\", got: %+v", rules)
	}
	rev, okRev := rules["acl-1-reverse#xs0"]
	if !okRev {
		t.Fatalf("expected bidirectional policy to add reverse rule acl-1-reverse#xs0, got: %+v", rules)
	}
	if len(rule.IPList) != 1 || rule.IPList[0].String() != "10.10.0.10/32" {
		t.Fatalf("expected src side to use selected ip only, got %v", rule.IPList)
	}
	if len(rule.Dst) != 1 || rule.Dst[0].String() != "10.20.0.0/24" {
		t.Fatalf("expected dst side to use full egress range fallback, got %v", rule.Dst)
	}
	if len(rev.IPList) != 1 || rev.IPList[0].String() != "10.20.0.0/24" {
		t.Fatalf("expected reverse rule src to be dst range, got %v", rev.IPList)
	}
	if len(rev.Dst) != 1 || rev.Dst[0].String() != "10.10.0.10/32" {
		t.Fatalf("expected reverse rule dst to be selected src ip, got %v", rev.Dst)
	}
}

// TestGetEgressAclRulesForTargetNode_UniEmitsForwardOnDstSideNode verifies that for
// a uni-directional egress-to-egress policy (src-egress -> dst-egress), the node
// hosting the DST egress also gets the forward firewall rule. Without it the
// packet arrives at the dst-egress router but its FORWARD chain has no matching
// allow, so traffic from the src LAN to the dst LAN is dropped at the gateway.
// The Bi case worked because its guard already permitted dst-side nodes.
func TestGetEgressAclRulesForTargetNode_UniEmitsForwardOnDstSideNode(t *testing.T) {
	originalGetEgressByID := getEgressByID
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
	})

	targetID := uuid.New()
	srcOwnerID := uuid.New()
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
		},
	}
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{
			{
				ID:               "acl-uni",
				Enabled:          true,
				AllowedDirection: models.TrafficDirectionUni,
				Proto:            models.ALL,
				Src:              []models.AclPolicyTag{{ID: models.EgressID, Value: "src-egress"}},
				Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "dst-egress"}},
			},
		}
	}
	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "src-egress":
			return schema.Egress{ID: egressID, Status: true, Range: "10.10.0.0/24", Nodes: datatypes.JSONMap{srcOwnerID.String(): true}}, nil
		case "dst-egress":
			return schema.Egress{ID: egressID, Status: true, Range: "10.20.0.0/24", Nodes: datatypes.JSONMap{targetID.String(): true}}, nil
		default:
			return schema.Egress{}, errors.New("not found")
		}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) { return nil, nil }

	rules := getEgressAclRulesForTargetNode(targetNode)
	fwd, ok := rules["acl-uni#xs0"]
	if !ok {
		t.Fatalf("expected uni forward rule on dst-side node, got: %+v", rules)
	}
	srcs := map[string]struct{}{}
	for _, n := range fwd.IPList {
		srcs[n.String()] = struct{}{}
	}
	if _, ok := srcs["10.10.0.0/24"]; !ok {
		t.Fatalf("expected forward IPList to contain src egress range 10.10.0.0/24, got %v", fwd.IPList)
	}
	dsts := map[string]struct{}{}
	for _, n := range fwd.Dst {
		dsts[n.String()] = struct{}{}
	}
	if _, ok := dsts["10.20.0.0/24"]; !ok {
		t.Fatalf("expected forward Dst to contain dst egress range 10.20.0.0/24, got %v", fwd.Dst)
	}
	// Uni must NOT emit a reverse leg on either side.
	if _, ok := rules["acl-uni-reverse#xs0"]; ok {
		t.Fatalf("did not expect reverse rule for uni-directional policy, rules: %+v", rules)
	}
}

// TestGetEgressAclRulesForTargetNode_UniSkipsUninvolvedNodes verifies that a node
// that hosts neither the src nor the dst egress gets no rules from this policy.
func TestGetEgressAclRulesForTargetNode_UniSkipsUninvolvedNodes(t *testing.T) {
	originalGetEgressByID := getEgressByID
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
	})

	// targetID is some random node hosting NEITHER src-egress nor dst-egress.
	targetID := uuid.New()
	srcOwnerID := uuid.New()
	dstOwnerID := uuid.New()
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
		},
	}
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{
			{
				ID:               "acl-uni",
				Enabled:          true,
				AllowedDirection: models.TrafficDirectionUni,
				Proto:            models.ALL,
				Src:              []models.AclPolicyTag{{ID: models.EgressID, Value: "src-egress"}},
				Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "dst-egress"}},
			},
		}
	}
	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "src-egress":
			return schema.Egress{ID: egressID, Status: true, Range: "10.10.0.0/24", Nodes: datatypes.JSONMap{srcOwnerID.String(): true}}, nil
		case "dst-egress":
			return schema.Egress{ID: egressID, Status: true, Range: "10.20.0.0/24", Nodes: datatypes.JSONMap{dstOwnerID.String(): true}}, nil
		default:
			return schema.Egress{}, errors.New("not found")
		}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) { return nil, nil }

	rules := getEgressAclRulesForTargetNode(targetNode)
	if len(rules) != 0 {
		t.Fatalf("expected no rules for node that routes neither src nor dst egress, got %d", len(rules))
	}
}

func TestGetEgressCIDRs_ReturnsV4AndV6Ranges(t *testing.T) {
	egresses := []schema.Egress{
		{Status: true, Range: "10.10.0.0/24"},
		{Status: true, Range: "fd00::/64"},
		{Status: false, Range: "10.30.0.0/24"},
	}
	v4, v6 := getEgressCIDRs(egresses)
	if len(v4) != 1 || len(v6) != 1 {
		t.Fatalf("expected one v4 and one v6 cidr, got %d and %d", len(v4), len(v6))
	}
	_, want4, _ := net.ParseCIDR("10.10.0.0/24")
	_, want6, _ := net.ParseCIDR("fd00::/64")
	if v4[0].String() != want4.String() || v6[0].String() != want6.String() {
		t.Fatalf("unexpected CIDRs returned: %v %v", v4, v6)
	}
}

func TestIsEgressRoutingPolicyAllowedForNodes_UniForwardAllowed(t *testing.T) {
	originalGetEgressByID := getEgressByID
	originalGetEgressByNetwork := getEgressByNetwork
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
		getEgressByNetwork = originalGetEgressByNetwork
	})

	nodeID := uuid.New()
	peerID := uuid.New()
	node := models.Node{CommonNode: models.CommonNode{ID: nodeID, Network: "netmaker"}}
	peer := models.Node{CommonNode: models.CommonNode{ID: peerID, Network: "netmaker"}}
	policy := models.Acl{
		Enabled:          true,
		AllowedDirection: models.TrafficDirectionUni,
		Src:              []models.AclPolicyTag{{ID: models.EgressID, Value: "src-egress"}},
		Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "dst-egress"}},
	}
	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "src-egress":
			return schema.Egress{ID: egressID, Status: true, Nodes: datatypes.JSONMap{nodeID.String(): true}}, nil
		case "dst-egress":
			return schema.Egress{ID: egressID, Status: true, Nodes: datatypes.JSONMap{peerID.String(): true}}, nil
		default:
			return schema.Egress{}, errors.New("not found")
		}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) { return nil, nil }

	if !IsEgressRoutingPolicyAllowedForNodes(policy, node, peer) {
		t.Fatal("expected uni-direction policy to allow forward src->dst routed peers")
	}
}

// TestIsEgressRoutingPolicyAllowedForNodes_UniReverseAlsoAllowedForPeering
// asserts that a Uni site-to-site policy permits the policy match in the
// reverse (dst-egress -> src-egress) orientation as well, because peering is
// bidirectional at the WireGuard tunnel level. Without this, the dst-side
// egress router queries IsPeerAllowed(dst, src) -> false and never adds the
// src-side egress router as a wg peer, so the handshake does not occur and
// the Uni L4 traffic itself never flows. The actual L4 direction is enforced
// downstream by the FORWARD/INPUT rule generators, not at peer-allow time.
func TestIsEgressRoutingPolicyAllowedForNodes_UniReverseAlsoAllowedForPeering(t *testing.T) {
	originalGetEgressByID := getEgressByID
	originalGetEgressByNetwork := getEgressByNetwork
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
		getEgressByNetwork = originalGetEgressByNetwork
	})

	nodeID := uuid.New()
	peerID := uuid.New()
	node := models.Node{CommonNode: models.CommonNode{ID: nodeID, Network: "netmaker"}}
	peer := models.Node{CommonNode: models.CommonNode{ID: peerID, Network: "netmaker"}}
	policy := models.Acl{
		Enabled:          true,
		AllowedDirection: models.TrafficDirectionUni,
		Src:              []models.AclPolicyTag{{ID: models.EgressID, Value: "src-egress"}},
		Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "dst-egress"}},
	}
	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "src-egress":
			return schema.Egress{ID: egressID, Status: true, Nodes: datatypes.JSONMap{peerID.String(): true}}, nil
		case "dst-egress":
			return schema.Egress{ID: egressID, Status: true, Nodes: datatypes.JSONMap{nodeID.String(): true}}, nil
		default:
			return schema.Egress{}, errors.New("not found")
		}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) { return nil, nil }

	if !IsEgressRoutingPolicyAllowedForNodes(policy, node, peer) {
		t.Fatal("expected uni-direction policy to allow reverse-orientation peer match (peering is bidirectional)")
	}
}

func TestIsEgressRoutingPolicyAllowedForNodes_BiReverseAllowed(t *testing.T) {
	originalGetEgressByID := getEgressByID
	originalGetEgressByNetwork := getEgressByNetwork
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
		getEgressByNetwork = originalGetEgressByNetwork
	})

	nodeID := uuid.New()
	peerID := uuid.New()
	node := models.Node{CommonNode: models.CommonNode{ID: nodeID, Network: "netmaker"}}
	peer := models.Node{CommonNode: models.CommonNode{ID: peerID, Network: "netmaker"}}
	policy := models.Acl{
		Enabled:          true,
		AllowedDirection: models.TrafficDirectionBi,
		Src:              []models.AclPolicyTag{{ID: models.EgressID, Value: "src-egress"}},
		Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "dst-egress"}},
	}
	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "src-egress":
			return schema.Egress{ID: egressID, Status: true, Nodes: datatypes.JSONMap{peerID.String(): true}}, nil
		case "dst-egress":
			return schema.Egress{ID: egressID, Status: true, Nodes: datatypes.JSONMap{nodeID.String(): true}}, nil
		default:
			return schema.Egress{}, errors.New("not found")
		}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) { return nil, nil }

	if !IsEgressRoutingPolicyAllowedForNodes(policy, node, peer) {
		t.Fatal("expected bi-direction policy to allow reverse routing participation")
	}
}

func TestIsEgressRoutingPolicyAllowedForNodes_NonEgressDenied(t *testing.T) {
	node := models.Node{CommonNode: models.CommonNode{ID: uuid.New(), Network: "netmaker"}}
	peer := models.Node{CommonNode: models.CommonNode{ID: uuid.New(), Network: "netmaker"}}
	policy := models.Acl{
		Enabled:          true,
		AllowedDirection: models.TrafficDirectionBi,
		Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "tag-a"}},
		Dst:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "tag-b"}},
	}
	if IsEgressRoutingPolicyAllowedForNodes(policy, node, peer) {
		t.Fatal("expected non-egress policy to be ignored by egress-routing helper")
	}
}

func TestAddEgressInfoToPeerByAccess_AllowsViaEgressRoutingPolicy(t *testing.T) {
	originalGetEgressByID := getEgressByID
	originalGetEgressByNetwork := getEgressByNetwork
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
		getEgressByNetwork = originalGetEgressByNetwork
	})

	nodeID := uuid.New()
	targetID := uuid.New()
	node := models.Node{
		CommonNode: models.CommonNode{ID: nodeID, Network: "netmaker"},
		Tags:       map[models.TagID]struct{}{},
	}
	target := models.Node{
		CommonNode: models.CommonNode{ID: targetID, Network: "netmaker"},
		Tags:       map[models.TagID]struct{}{},
	}

	srcEgress := schema.Egress{
		ID:     "src-egress",
		Status: true,
		Network: "netmaker",
		Range:  "10.10.0.0/24",
		Nodes: datatypes.JSONMap{
			nodeID.String(): json.Number("100"),
		},
	}
	dstEgress := schema.Egress{
		ID:     "dst-egress",
		Status: true,
		Network: "netmaker",
		Range:  "10.20.0.0/24",
		Nodes: datatypes.JSONMap{
			targetID.String(): json.Number("100"),
		},
	}
	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "src-egress":
			return srcEgress, nil
		case "dst-egress":
			return dstEgress, nil
		default:
			return schema.Egress{}, errors.New("not found")
		}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) { return nil, nil }

	acls := []models.Acl{{
		Enabled:          true,
		AllowedDirection: models.TrafficDirectionUni,
		Src:              []models.AclPolicyTag{{ID: models.EgressID, Value: "src-egress"}},
		Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "dst-egress"}},
	}}
	AddEgressInfoToPeerByAccess(&node, &target, []schema.Egress{dstEgress}, acls, false)

	if !target.EgressDetails.IsEgressGateway {
		t.Fatal("expected target to receive egress info via egress-to-egress routing policy")
	}
	if len(target.EgressDetails.EgressGatewayRanges) == 0 || target.EgressDetails.EgressGatewayRanges[0] != "10.20.0.0/24" {
		t.Fatalf("expected dst egress range to be assigned, got %v", target.EgressDetails.EgressGatewayRanges)
	}
}

func TestAddEgressInfoToPeerByAccess_DeniesUniReverseRouting(t *testing.T) {
	originalGetEgressByID := getEgressByID
	originalGetEgressByNetwork := getEgressByNetwork
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
		getEgressByNetwork = originalGetEgressByNetwork
	})

	nodeID := uuid.New()
	targetID := uuid.New()
	node := models.Node{
		CommonNode: models.CommonNode{ID: nodeID, Network: "netmaker"},
		Tags:       map[models.TagID]struct{}{},
	}
	target := models.Node{
		CommonNode: models.CommonNode{ID: targetID, Network: "netmaker"},
		Tags:       map[models.TagID]struct{}{},
	}

	srcEgress := schema.Egress{
		ID:     "src-egress",
		Status: true,
		Network: "netmaker",
		Range:  "10.10.0.0/24",
		Nodes: datatypes.JSONMap{
			targetID.String(): json.Number("100"),
		},
	}
	dstEgress := schema.Egress{
		ID:     "dst-egress",
		Status: true,
		Network: "netmaker",
		Range:  "10.20.0.0/24",
		Nodes: datatypes.JSONMap{
			nodeID.String(): json.Number("100"),
		},
	}
	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "src-egress":
			return srcEgress, nil
		case "dst-egress":
			return dstEgress, nil
		default:
			return schema.Egress{}, errors.New("not found")
		}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) { return nil, nil }

	acls := []models.Acl{{
		Enabled:          true,
		AllowedDirection: models.TrafficDirectionUni,
		Src:              []models.AclPolicyTag{{ID: models.EgressID, Value: "src-egress"}},
		Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "dst-egress"}},
	}}
	AddEgressInfoToPeerByAccess(&node, &target, []schema.Egress{srcEgress}, acls, false)

	if target.EgressDetails.IsEgressGateway {
		t.Fatalf("expected no egress info for uni reverse-only routing, got %+v", target.EgressDetails)
	}
}

func TestIsEgressToEgressPolicyForTarget_NoRoutingMatch(t *testing.T) {
	originalGetEgressByID := getEgressByID
	originalGetEgressByNetwork := getEgressByNetwork
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
		getEgressByNetwork = originalGetEgressByNetwork
	})

	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      uuid.New(),
			Network: "netmaker",
		},
	}
	policy := models.Acl{
		Enabled: true,
		Src:     []models.AclPolicyTag{{ID: models.EgressID, Value: "src-egress"}},
		Dst:     []models.AclPolicyTag{{ID: models.EgressID, Value: "dst-egress"}},
	}
	getEgressByID = func(egressID string) (schema.Egress, error) {
		return schema.Egress{
			ID:     egressID,
			Status: true,
			Nodes: datatypes.JSONMap{
				uuid.New().String(): true,
			},
		}, nil
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return nil, nil
	}

	if isEgressToEgressPolicyForTarget(policy, targetNode) {
		t.Fatal("expected policy not to match when target node routes neither source nor destination egress")
	}
}

func TestGetEgressAclRulesForTargetNode_NatUsesMeshSrcOnDstRoutingNode(t *testing.T) {
	originalGetEgressByID := getEgressByID
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalGetNodeByIDForEgressFw := getNodeByIDForEgressFw
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		getNodeByIDForEgressFw = originalGetNodeByIDForEgressFw
	})

	targetID := uuid.New()
	routerID := uuid.New()
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
		},
		Tags: map[models.TagID]struct{}{},
	}

	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{
			{
				ID:               "acl-nat",
				Enabled:          true,
				AllowedDirection: models.TrafficDirectionBi,
				Proto:            models.ALL,
				Src: []models.AclPolicyTag{
					{ID: models.EgressID, Value: "src-egress"},
					{ID: models.NetmakerIPAclID, Value: "10.10.0.10/32"},
				},
				Dst: []models.AclPolicyTag{
					{ID: models.EgressID, Value: "dst-egress"},
				},
			},
		}
	}
	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "src-egress":
			return schema.Egress{
				ID:     egressID,
				Status: true,
				Nat:    true,
				Range:  "10.10.0.0/24",
				Nodes: datatypes.JSONMap{
					routerID.String(): json.Number("100"),
				},
			}, nil
		case "dst-egress":
			return schema.Egress{
				ID:     egressID,
				Status: true,
				Range:  "10.20.0.0/24",
				Nodes: datatypes.JSONMap{
					targetID.String(): json.Number("100"),
				},
			}, nil
		default:
			return schema.Egress{}, errors.New("not found")
		}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) { return nil, nil }

	getNodeByIDForEgressFw = func(nodeID string) (models.Node, error) {
		if nodeID != routerID.String() {
			return models.Node{}, errors.New("not found")
		}
		return models.Node{
			CommonNode: models.CommonNode{
				ID:      routerID,
				Network: "netmaker",
				Address: net.IPNet{
					IP:   net.ParseIP("10.110.0.112"),
					Mask: net.CIDRMask(32, 32),
				},
			},
		}, nil
	}

	rules := getEgressAclRulesForTargetNode(targetNode)
	fwd, ok := rules["acl-nat#xs0"]
	if !ok {
		t.Fatalf("expected forward rule acl-nat#xs0, got: %+v", rules)
	}
	if len(fwd.IPList) != 1 || fwd.IPList[0].IP.String() != "10.110.0.112" {
		t.Fatalf("expected forward ip_list to use peer mesh IP when src egress NAT is on, got %v", fwd.IPList)
	}
}

func TestGetEgressRulesForNode_BiPolicyEmitsExplicitReverseRule(t *testing.T) {
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalGetTagMap := GetTagMapWithNodesByNetwork
	t.Cleanup(func() {
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		GetTagMapWithNodesByNetwork = originalGetTagMap
	})

	targetID := uuid.New()
	peerID := uuid.New()
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
			Address: net.IPNet{
				IP:   net.ParseIP("100.64.0.5"),
				Mask: net.CIDRMask(32, 32),
			},
		},
		Tags: map[models.TagID]struct{}{},
	}

	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return []schema.Egress{{
			ID:      "egress-1",
			Network: network,
			Status:  true,
			Range:   "10.104.0.0/20",
			Nodes:   datatypes.JSONMap{targetID.String(): json.Number("100")},
		}}, nil
	}
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "acl-bi-dev-egress",
			Enabled:          true,
			AllowedDirection: models.TrafficDirectionBi,
			Proto:            models.ALL,
			Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "snowy-waterfall"}},
			Dst: []models.AclPolicyTag{
				{ID: models.EgressID, Value: "egress-1"},
				{ID: models.NetmakerIPAclID, Value: "10.104.0.1/32"},
				{ID: models.NetmakerIPAclID, Value: "10.104.0.16/32"},
			},
		}}
	}
	GetTagMapWithNodesByNetwork = func(netID schema.NetworkID, withStatic bool) map[models.TagID][]models.Node {
		return map[models.TagID][]models.Node{
			"snowy-waterfall": {{
				CommonNode: models.CommonNode{
					ID: peerID,
					Address: net.IPNet{
						IP:   net.ParseIP("100.64.0.7"),
						Mask: net.CIDRMask(32, 32),
					},
				},
			}},
		}
	}

	rules := GetEgressRulesForNode(targetNode)
	fwd, ok := rules["acl-bi-dev-egress"]
	if !ok {
		t.Fatalf("expected forward rule keyed by acl.ID, got: %+v", rules)
	}
	fwdSrcSet := make(map[string]struct{}, len(fwd.IPList))
	for _, n := range fwd.IPList {
		fwdSrcSet[n.String()] = struct{}{}
	}
	if _, ok := fwdSrcSet["100.64.0.7/32"]; !ok {
		t.Fatalf("expected forward IPList to contain device peer /32, got %v", fwd.IPList)
	}
	fwdDstSet := make(map[string]struct{}, len(fwd.Dst))
	for _, n := range fwd.Dst {
		fwdDstSet[n.String()] = struct{}{}
	}
	if _, ok := fwdDstSet["10.104.0.1/32"]; !ok {
		t.Fatalf("expected forward Dst to contain selected egress IP 10.104.0.1/32, got %v", fwd.Dst)
	}
	if _, ok := fwdDstSet["10.104.0.16/32"]; !ok {
		t.Fatalf("expected forward Dst to contain selected egress IP 10.104.0.16/32, got %v", fwd.Dst)
	}

	rev, ok := rules["acl-bi-dev-egress-reverse"]
	if !ok {
		t.Fatalf("expected explicit reverse rule keyed by acl.ID + \"-reverse\" for Bi policy, got: %+v", rules)
	}
	revSrcSet := make(map[string]struct{}, len(rev.IPList))
	for _, n := range rev.IPList {
		revSrcSet[n.String()] = struct{}{}
	}
	if _, ok := revSrcSet["10.104.0.1/32"]; !ok {
		t.Fatalf("expected reverse IPList to be the forward Dst (egress IPs), missing 10.104.0.1/32, got %v", rev.IPList)
	}
	if _, ok := revSrcSet["10.104.0.16/32"]; !ok {
		t.Fatalf("expected reverse IPList to contain 10.104.0.16/32, got %v", rev.IPList)
	}
	revDstSet := make(map[string]struct{}, len(rev.Dst))
	for _, n := range rev.Dst {
		revDstSet[n.String()] = struct{}{}
	}
	if _, ok := revDstSet["100.64.0.7/32"]; !ok {
		t.Fatalf("expected reverse Dst to be the forward IPList (device peer /32), got %v", rev.Dst)
	}
}

func TestGetEgressRulesForNode_UniPolicyDoesNotEmitReverseRule(t *testing.T) {
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalGetTagMap := GetTagMapWithNodesByNetwork
	t.Cleanup(func() {
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		GetTagMapWithNodesByNetwork = originalGetTagMap
	})

	targetID := uuid.New()
	peerID := uuid.New()
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
			Address: net.IPNet{
				IP:   net.ParseIP("100.64.0.5"),
				Mask: net.CIDRMask(32, 32),
			},
		},
		Tags: map[models.TagID]struct{}{},
	}

	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return []schema.Egress{{
			ID:      "egress-1",
			Network: network,
			Status:  true,
			Range:   "10.104.0.0/20",
			Nodes:   datatypes.JSONMap{targetID.String(): json.Number("100")},
		}}, nil
	}
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "acl-uni-dev-egress",
			Enabled:          true,
			AllowedDirection: models.TrafficDirectionUni,
			Proto:            models.ALL,
			Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "snowy-waterfall"}},
			Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "egress-1"}},
		}}
	}
	GetTagMapWithNodesByNetwork = func(netID schema.NetworkID, withStatic bool) map[models.TagID][]models.Node {
		return map[models.TagID][]models.Node{
			"snowy-waterfall": {{
				CommonNode: models.CommonNode{
					ID: peerID,
					Address: net.IPNet{
						IP:   net.ParseIP("100.64.0.7"),
						Mask: net.CIDRMask(32, 32),
					},
				},
			}},
		}
	}

	rules := GetEgressRulesForNode(targetNode)
	if _, ok := rules["acl-uni-dev-egress"]; !ok {
		t.Fatalf("expected forward rule for uni policy, got: %+v", rules)
	}
	if _, ok := rules["acl-uni-dev-egress-reverse"]; ok {
		t.Fatalf("did not expect explicit reverse rule for uni-directional policy, rules: %+v", rules)
	}
}

// TestGetEgressRulesForNode_RemoteEgressEmitsExtclientFwdRule verifies that when an
// extclient is attached to targetnode and a policy grants it access to an egress
// hosted on a DIFFERENT node, targetnode's forward-chain rules include the
// extclient -> remote egress range allow. Without this rule the packet is dropped
// at targetnode even though the ingress-chain permits it.
func TestGetEgressRulesForNode_RemoteEgressEmitsExtclientFwdRule(t *testing.T) {
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalGetTagMap := GetTagMapWithNodesByNetwork
	originalListExtClients := listNetworkExtClients
	t.Cleanup(func() {
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		GetTagMapWithNodesByNetwork = originalGetTagMap
		listNetworkExtClients = originalListExtClients
	})

	targetID := uuid.New()
	remoteOwnerID := uuid.New()
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
			Address: net.IPNet{
				IP:   net.ParseIP("100.64.0.5"),
				Mask: net.CIDRMask(32, 32),
			},
		},
		Tags: map[models.TagID]struct{}{},
	}

	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return []schema.Egress{
			{
				ID:      "egress-local",
				Network: network,
				Status:  true,
				Range:   "10.50.0.0/24",
				Nodes:   datatypes.JSONMap{targetID.String(): json.Number("100")},
			},
			{
				ID:      "egress-remote",
				Network: network,
				Status:  true,
				Range:   "10.20.0.0/24",
				Nodes:   datatypes.JSONMap{remoteOwnerID.String(): json.Number("100")},
			},
		}, nil
	}
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "acl-ext-to-remote-egress",
			Enabled:          true,
			AllowedDirection: models.TrafficDirectionUni,
			Proto:            models.ALL,
			Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "marketing"}},
			Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "egress-remote"}},
		}}
	}
	GetTagMapWithNodesByNetwork = func(netID schema.NetworkID, withStatic bool) map[models.TagID][]models.Node {
		return map[models.TagID][]models.Node{}
	}
	listNetworkExtClients = func(network string) ([]models.ExtClient, error) {
		return []models.ExtClient{
			{
				ClientID:         "ec-1",
				Network:          "netmaker",
				Enabled:          true,
				IngressGatewayID: targetID.String(),
				Address:          "100.64.0.10",
				Tags: map[models.TagID]struct{}{
					"marketing": {},
				},
			},
		}, nil
	}

	rules := GetEgressRulesForNode(targetNode)
	fwd, ok := rules["acl-ext-to-remote-egress#ext-fwd"]
	if !ok {
		t.Fatalf("expected ext-fwd rule for remote egress, got rules: %+v", rules)
	}
	srcSet := make(map[string]struct{}, len(fwd.IPList))
	for _, n := range fwd.IPList {
		srcSet[n.String()] = struct{}{}
	}
	if _, ok := srcSet["100.64.0.10/32"]; !ok {
		t.Fatalf("expected ext-fwd IPList to contain attached extclient /32, got %v", fwd.IPList)
	}
	dstSet := make(map[string]struct{}, len(fwd.Dst))
	for _, n := range fwd.Dst {
		dstSet[n.String()] = struct{}{}
	}
	if _, ok := dstSet["10.20.0.0/24"]; !ok {
		t.Fatalf("expected ext-fwd Dst to contain remote egress range 10.20.0.0/24, got %v", fwd.Dst)
	}
	if _, ok := rules["acl-ext-to-remote-egress#ext-fwd-reverse"]; ok {
		t.Fatalf("did not expect reverse rule for uni-directional policy, rules: %+v", rules)
	}
}

// TestGetEgressRulesForNode_RemoteEgressBiEmitsReverse verifies that Bi-directional
// policies emit a -reverse companion so return traffic from the remote egress range
// to the extclient is also allowed at targetnode's forward chain.
func TestGetEgressRulesForNode_RemoteEgressBiEmitsReverse(t *testing.T) {
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalGetTagMap := GetTagMapWithNodesByNetwork
	originalListExtClients := listNetworkExtClients
	t.Cleanup(func() {
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		GetTagMapWithNodesByNetwork = originalGetTagMap
		listNetworkExtClients = originalListExtClients
	})

	targetID := uuid.New()
	remoteOwnerID := uuid.New()
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
		},
		Tags: map[models.TagID]struct{}{},
	}

	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return []schema.Egress{
			{
				ID:      "egress-local",
				Network: network,
				Status:  true,
				Range:   "10.50.0.0/24",
				Nodes:   datatypes.JSONMap{targetID.String(): json.Number("100")},
			},
			{
				ID:      "egress-remote",
				Network: network,
				Status:  true,
				Range:   "10.20.0.0/24",
				Nodes:   datatypes.JSONMap{remoteOwnerID.String(): json.Number("100")},
			},
		}, nil
	}
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "acl-ext-bi",
			Enabled:          true,
			AllowedDirection: models.TrafficDirectionBi,
			Proto:            models.ALL,
			Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "marketing"}},
			Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "egress-remote"}},
		}}
	}
	GetTagMapWithNodesByNetwork = func(netID schema.NetworkID, withStatic bool) map[models.TagID][]models.Node {
		return map[models.TagID][]models.Node{}
	}
	listNetworkExtClients = func(network string) ([]models.ExtClient, error) {
		return []models.ExtClient{
			{
				ClientID:         "ec-1",
				Network:          "netmaker",
				Enabled:          true,
				IngressGatewayID: targetID.String(),
				Address:          "100.64.0.10",
				Tags: map[models.TagID]struct{}{
					"marketing": {},
				},
			},
		}, nil
	}

	rules := GetEgressRulesForNode(targetNode)
	rev, ok := rules["acl-ext-bi#ext-fwd-reverse"]
	if !ok {
		t.Fatalf("expected -reverse companion for Bi ext-fwd, rules: %+v", rules)
	}
	revSrcSet := make(map[string]struct{}, len(rev.IPList))
	for _, n := range rev.IPList {
		revSrcSet[n.String()] = struct{}{}
	}
	if _, ok := revSrcSet["10.20.0.0/24"]; !ok {
		t.Fatalf("expected reverse IPList to contain remote egress range, got %v", rev.IPList)
	}
	revDstSet := make(map[string]struct{}, len(rev.Dst))
	for _, n := range rev.Dst {
		revDstSet[n.String()] = struct{}{}
	}
	if _, ok := revDstSet["100.64.0.10/32"]; !ok {
		t.Fatalf("expected reverse Dst to contain attached extclient /32, got %v", rev.Dst)
	}
}

// TestGetExtClientEgressFwRulesOnIngressGw_RemoteEgressEmitsFwRule verifies that the
// ingress gateway's forward chain receives an explicit allow rule for an attached
// extclient -> remote egress range, so the packet is not dropped at this gateway
// (relevant when the ingress gw is not itself an egress gw and GetEgressRulesForNode
// is never called).
func TestGetExtClientEgressFwRulesOnIngressGw_RemoteEgressEmitsFwRule(t *testing.T) {
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalListExtClients := listNetworkExtClients
	t.Cleanup(func() {
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		listNetworkExtClients = originalListExtClients
	})

	targetID := uuid.New()
	remoteOwnerID := uuid.New()
	node := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
		},
	}

	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return []schema.Egress{{
			ID:      "egress-remote",
			Network: network,
			Status:  true,
			Range:   "10.20.0.0/24",
			Nodes:   datatypes.JSONMap{remoteOwnerID.String(): json.Number("100")},
		}}, nil
	}
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "acl-ext-to-remote",
			Enabled:          true,
			AllowedDirection: models.TrafficDirectionUni,
			Proto:            models.ALL,
			Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "marketing"}},
			Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "egress-remote"}},
		}}
	}
	listNetworkExtClients = func(network string) ([]models.ExtClient, error) {
		return []models.ExtClient{{
			ClientID:         "ec-1",
			Network:          "netmaker",
			Enabled:          true,
			IngressGatewayID: targetID.String(),
			Address:          "100.64.0.10",
			Tags:             map[models.TagID]struct{}{"marketing": {}},
		}}, nil
	}

	rules := getExtClientEgressFwRulesOnIngressGw(node)
	if len(rules) == 0 {
		t.Fatalf("expected at least one fw rule for attached ext -> remote egress, got 0")
	}
	var found bool
	for _, r := range rules {
		if r.SrcIP.String() == "100.64.0.10/32" && r.DstIP.String() == "10.20.0.0/24" && r.Allow {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected EC 100.64.0.10/32 -> 10.20.0.0/24 allow rule, got: %+v", rules)
	}
	for _, r := range rules {
		if r.SrcIP.String() == "10.20.0.0/24" && r.DstIP.String() == "100.64.0.10/32" {
			t.Fatalf("did not expect reverse rule for uni-directional policy, got: %+v", rules)
		}
	}
}

// TestGetExtClientEgressFwRulesOnIngressGw_BiEmitsReverse verifies the Bi-directional
// case emits both EC -> egress range AND egress range -> EC on the forward chain.
func TestGetExtClientEgressFwRulesOnIngressGw_BiEmitsReverse(t *testing.T) {
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalListExtClients := listNetworkExtClients
	t.Cleanup(func() {
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		listNetworkExtClients = originalListExtClients
	})

	targetID := uuid.New()
	remoteOwnerID := uuid.New()
	node := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
		},
	}

	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return []schema.Egress{{
			ID:      "egress-remote",
			Network: network,
			Status:  true,
			Range:   "10.20.0.0/24",
			Nodes:   datatypes.JSONMap{remoteOwnerID.String(): json.Number("100")},
		}}, nil
	}
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "acl-ext-bi",
			Enabled:          true,
			AllowedDirection: models.TrafficDirectionBi,
			Proto:            models.ALL,
			Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "marketing"}},
			Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "egress-remote"}},
		}}
	}
	listNetworkExtClients = func(network string) ([]models.ExtClient, error) {
		return []models.ExtClient{{
			ClientID:         "ec-1",
			Network:          "netmaker",
			Enabled:          true,
			IngressGatewayID: targetID.String(),
			Address:          "100.64.0.10",
			Tags:             map[models.TagID]struct{}{"marketing": {}},
		}}, nil
	}

	rules := getExtClientEgressFwRulesOnIngressGw(node)
	var fwd, rev bool
	for _, r := range rules {
		if r.SrcIP.String() == "100.64.0.10/32" && r.DstIP.String() == "10.20.0.0/24" {
			fwd = true
		}
		if r.SrcIP.String() == "10.20.0.0/24" && r.DstIP.String() == "100.64.0.10/32" {
			rev = true
		}
	}
	if !fwd {
		t.Fatalf("expected forward rule EC -> remote egress range, got: %+v", rules)
	}
	if !rev {
		t.Fatalf("expected reverse rule remote egress range -> EC for Bi policy, got: %+v", rules)
	}
}

// TestGetExtClientEgressFwRulesOnIngressGw_IgnoresExtclientsOnOtherGw verifies that
// extclients attached to a different ingress gw produce no rules here; their traffic
// does not flow through this gateway.
func TestGetExtClientEgressFwRulesOnIngressGw_IgnoresExtclientsOnOtherGw(t *testing.T) {
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalListExtClients := listNetworkExtClients
	t.Cleanup(func() {
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		listNetworkExtClients = originalListExtClients
	})

	targetID := uuid.New()
	otherGwID := uuid.New()
	remoteOwnerID := uuid.New()
	node := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
		},
	}

	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return []schema.Egress{{
			ID:      "egress-remote",
			Network: network,
			Status:  true,
			Range:   "10.20.0.0/24",
			Nodes:   datatypes.JSONMap{remoteOwnerID.String(): json.Number("100")},
		}}, nil
	}
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "acl-ext-to-remote",
			Enabled:          true,
			AllowedDirection: models.TrafficDirectionUni,
			Proto:            models.ALL,
			Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "marketing"}},
			Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "egress-remote"}},
		}}
	}
	listNetworkExtClients = func(network string) ([]models.ExtClient, error) {
		return []models.ExtClient{{
			ClientID:         "ec-on-other-gw",
			Network:          "netmaker",
			Enabled:          true,
			IngressGatewayID: otherGwID.String(),
			Address:          "100.64.0.99",
			Tags:             map[models.TagID]struct{}{"marketing": {}},
		}}, nil
	}

	rules := getExtClientEgressFwRulesOnIngressGw(node)
	if len(rules) != 0 {
		t.Fatalf("expected no fw rules when no extclient is attached to node, got: %+v", rules)
	}
}

// TestGetEgressRulesForNode_RemoteEgressIgnoresOtherGwAttachedExtclients verifies that
// extclients attached to a different gateway are NOT added to the ext-fwd IPList:
// their traffic does not flow through targetnode, so we must not author rules for them.
func TestGetEgressRulesForNode_RemoteEgressIgnoresOtherGwAttachedExtclients(t *testing.T) {
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalGetTagMap := GetTagMapWithNodesByNetwork
	originalListExtClients := listNetworkExtClients
	t.Cleanup(func() {
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		GetTagMapWithNodesByNetwork = originalGetTagMap
		listNetworkExtClients = originalListExtClients
	})

	targetID := uuid.New()
	otherGwID := uuid.New()
	remoteOwnerID := uuid.New()
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
		},
		Tags: map[models.TagID]struct{}{},
	}

	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return []schema.Egress{
			{
				ID:      "egress-local",
				Network: network,
				Status:  true,
				Range:   "10.50.0.0/24",
				Nodes:   datatypes.JSONMap{targetID.String(): json.Number("100")},
			},
			{
				ID:      "egress-remote",
				Network: network,
				Status:  true,
				Range:   "10.20.0.0/24",
				Nodes:   datatypes.JSONMap{remoteOwnerID.String(): json.Number("100")},
			},
		}, nil
	}
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "acl-ext-to-remote-egress",
			Enabled:          true,
			AllowedDirection: models.TrafficDirectionUni,
			Proto:            models.ALL,
			Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "marketing"}},
			Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "egress-remote"}},
		}}
	}
	GetTagMapWithNodesByNetwork = func(netID schema.NetworkID, withStatic bool) map[models.TagID][]models.Node {
		return map[models.TagID][]models.Node{}
	}
	listNetworkExtClients = func(network string) ([]models.ExtClient, error) {
		return []models.ExtClient{
			{
				ClientID:         "ec-on-other-gw",
				Network:          "netmaker",
				Enabled:          true,
				IngressGatewayID: otherGwID.String(),
				Address:          "100.64.0.99",
				Tags: map[models.TagID]struct{}{
					"marketing": {},
				},
			},
		}, nil
	}

	rules := GetEgressRulesForNode(targetNode)
	if _, ok := rules["acl-ext-to-remote-egress#ext-fwd"]; ok {
		t.Fatalf("did not expect ext-fwd rule when no extclient is attached to targetnode, rules: %+v", rules)
	}
}

// TestGetDeviceEgressFwRulesOnIngressGw_RemoteEgressEmitsFwRule verifies the relayed
// mesh device twin: a node relayed by this gateway with policy access to an egress
// hosted on a DIFFERENT node gets an explicit forward-chain allow on this gateway.
// Without this the device's traffic to the remote egress would be dropped at the
// relay since the blanket NetworkRange<->relayed rule only covers in-mesh traffic.
func TestGetDeviceEgressFwRulesOnIngressGw_RemoteEgressEmitsFwRule(t *testing.T) {
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalGetNodeByID := getNodeByID
	t.Cleanup(func() {
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		getNodeByID = originalGetNodeByID
	})

	targetID := uuid.New()
	relayedID := uuid.New()
	remoteOwnerID := uuid.New()
	relayedNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      relayedID,
			Network: "netmaker",
			Address: net.IPNet{
				IP:   net.ParseIP("100.64.0.20"),
				Mask: net.CIDRMask(32, 32),
			},
		},
		Tags: map[models.TagID]struct{}{"marketing": {}},
	}
	node := models.Node{
		CommonNode: models.CommonNode{
			ID:           targetID,
			Network:      "netmaker",
			RelayedNodes: []string{relayedID.String()},
		},
	}

	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return []schema.Egress{{
			ID:      "egress-remote",
			Network: network,
			Status:  true,
			Range:   "10.20.0.0/24",
			Nodes:   datatypes.JSONMap{remoteOwnerID.String(): json.Number("100")},
		}}, nil
	}
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "acl-dev-to-remote",
			Enabled:          true,
			AllowedDirection: models.TrafficDirectionUni,
			Proto:            models.ALL,
			Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "marketing"}},
			Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "egress-remote"}},
		}}
	}
	getNodeByID = func(id string) (models.Node, error) {
		if id == relayedID.String() {
			return relayedNode, nil
		}
		return models.Node{}, fmt.Errorf("not found")
	}

	rules := getDeviceEgressFwRulesOnIngressGw(node)
	if len(rules) == 0 {
		t.Fatalf("expected at least one fw rule for relayed device -> remote egress, got 0")
	}
	var found bool
	for _, r := range rules {
		if r.SrcIP.String() == "100.64.0.20/32" && r.DstIP.String() == "10.20.0.0/24" && r.Allow {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected relayed-device 100.64.0.20/32 -> 10.20.0.0/24 allow rule, got: %+v", rules)
	}
	for _, r := range rules {
		if r.SrcIP.String() == "10.20.0.0/24" && r.DstIP.String() == "100.64.0.20/32" {
			t.Fatalf("did not expect reverse rule for uni-directional policy, got: %+v", rules)
		}
	}
}

// TestGetDeviceEgressFwRulesOnIngressGw_BiEmitsReverse verifies Bi policies emit both
// forward and reverse rules at the relay's forward chain for relayed devices.
func TestGetDeviceEgressFwRulesOnIngressGw_BiEmitsReverse(t *testing.T) {
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalGetNodeByID := getNodeByID
	t.Cleanup(func() {
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		getNodeByID = originalGetNodeByID
	})

	targetID := uuid.New()
	relayedID := uuid.New()
	remoteOwnerID := uuid.New()
	relayedNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      relayedID,
			Network: "netmaker",
			Address: net.IPNet{
				IP:   net.ParseIP("100.64.0.20"),
				Mask: net.CIDRMask(32, 32),
			},
		},
		Tags: map[models.TagID]struct{}{"marketing": {}},
	}
	node := models.Node{
		CommonNode: models.CommonNode{
			ID:           targetID,
			Network:      "netmaker",
			RelayedNodes: []string{relayedID.String()},
		},
	}

	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return []schema.Egress{{
			ID:      "egress-remote",
			Network: network,
			Status:  true,
			Range:   "10.20.0.0/24",
			Nodes:   datatypes.JSONMap{remoteOwnerID.String(): json.Number("100")},
		}}, nil
	}
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "acl-dev-bi",
			Enabled:          true,
			AllowedDirection: models.TrafficDirectionBi,
			Proto:            models.ALL,
			Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "marketing"}},
			Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "egress-remote"}},
		}}
	}
	getNodeByID = func(id string) (models.Node, error) {
		if id == relayedID.String() {
			return relayedNode, nil
		}
		return models.Node{}, fmt.Errorf("not found")
	}

	rules := getDeviceEgressFwRulesOnIngressGw(node)
	var fwd, rev bool
	for _, r := range rules {
		if r.SrcIP.String() == "100.64.0.20/32" && r.DstIP.String() == "10.20.0.0/24" {
			fwd = true
		}
		if r.SrcIP.String() == "10.20.0.0/24" && r.DstIP.String() == "100.64.0.20/32" {
			rev = true
		}
	}
	if !fwd {
		t.Fatalf("expected forward rule relayed-device -> remote egress, got: %+v", rules)
	}
	if !rev {
		t.Fatalf("expected reverse rule remote egress -> relayed-device for Bi policy, got: %+v", rules)
	}
}

// TestGetDeviceEgressFwRulesOnIngressGw_IgnoresUnrelayedNodes verifies that mesh
// devices NOT in node.RelayedNodes do not produce rules here: their traffic does
// not flow through this gateway, so we must not author rules for them.
func TestGetDeviceEgressFwRulesOnIngressGw_IgnoresUnrelayedNodes(t *testing.T) {
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalGetNodeByID := getNodeByID
	t.Cleanup(func() {
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		getNodeByID = originalGetNodeByID
	})

	targetID := uuid.New()
	remoteOwnerID := uuid.New()
	// node has no RelayedNodes; a matching policy/egress exists but should be ignored.
	node := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
		},
	}

	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return []schema.Egress{{
			ID:      "egress-remote",
			Network: network,
			Status:  true,
			Range:   "10.20.0.0/24",
			Nodes:   datatypes.JSONMap{remoteOwnerID.String(): json.Number("100")},
		}}, nil
	}
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "acl-dev-to-remote",
			Enabled:          true,
			AllowedDirection: models.TrafficDirectionUni,
			Proto:            models.ALL,
			Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "marketing"}},
			Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "egress-remote"}},
		}}
	}
	getNodeByID = func(id string) (models.Node, error) {
		t.Fatalf("getNodeByID should not be called when node has no relayed nodes")
		return models.Node{}, nil
	}

	rules := getDeviceEgressFwRulesOnIngressGw(node)
	if len(rules) != 0 {
		t.Fatalf("expected no fw rules when node relays nobody, got: %+v", rules)
	}
}

// TestGetEgressRulesForNode_RemoteEgressEmitsDeviceFwdRule verifies that when a
// relayed mesh device has policy access to an egress hosted on a DIFFERENT node,
// targetnode's egress forward rules include the device -> remote egress range
// allow keyed with "#dev-fwd".
func TestGetEgressRulesForNode_RemoteEgressEmitsDeviceFwdRule(t *testing.T) {
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalGetTagMap := GetTagMapWithNodesByNetwork
	originalListExtClients := listNetworkExtClients
	originalGetNodeByID := getNodeByID
	t.Cleanup(func() {
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		GetTagMapWithNodesByNetwork = originalGetTagMap
		listNetworkExtClients = originalListExtClients
		getNodeByID = originalGetNodeByID
	})

	targetID := uuid.New()
	relayedID := uuid.New()
	remoteOwnerID := uuid.New()
	relayedNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      relayedID,
			Network: "netmaker",
			Address: net.IPNet{
				IP:   net.ParseIP("100.64.0.20"),
				Mask: net.CIDRMask(32, 32),
			},
		},
		Tags: map[models.TagID]struct{}{"marketing": {}},
	}
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:           targetID,
			Network:      "netmaker",
			RelayedNodes: []string{relayedID.String()},
		},
		Tags: map[models.TagID]struct{}{},
	}

	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return []schema.Egress{
			{
				ID:      "egress-local",
				Network: network,
				Status:  true,
				Range:   "10.50.0.0/24",
				Nodes:   datatypes.JSONMap{targetID.String(): json.Number("100")},
			},
			{
				ID:      "egress-remote",
				Network: network,
				Status:  true,
				Range:   "10.20.0.0/24",
				Nodes:   datatypes.JSONMap{remoteOwnerID.String(): json.Number("100")},
			},
		}, nil
	}
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "acl-dev-to-remote-egress",
			Enabled:          true,
			AllowedDirection: models.TrafficDirectionUni,
			Proto:            models.ALL,
			Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "marketing"}},
			Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "egress-remote"}},
		}}
	}
	GetTagMapWithNodesByNetwork = func(netID schema.NetworkID, withStatic bool) map[models.TagID][]models.Node {
		return map[models.TagID][]models.Node{}
	}
	listNetworkExtClients = func(network string) ([]models.ExtClient, error) {
		return nil, nil
	}
	getNodeByID = func(id string) (models.Node, error) {
		if id == relayedID.String() {
			return relayedNode, nil
		}
		return models.Node{}, fmt.Errorf("not found")
	}

	rules := GetEgressRulesForNode(targetNode)
	fwd, ok := rules["acl-dev-to-remote-egress#dev-fwd"]
	if !ok {
		t.Fatalf("expected dev-fwd rule for remote egress, got rules: %+v", rules)
	}
	srcSet := make(map[string]struct{}, len(fwd.IPList))
	for _, n := range fwd.IPList {
		srcSet[n.String()] = struct{}{}
	}
	if _, ok := srcSet["100.64.0.20/32"]; !ok {
		t.Fatalf("expected dev-fwd IPList to contain relayed device /32, got %v", fwd.IPList)
	}
	dstSet := make(map[string]struct{}, len(fwd.Dst))
	for _, n := range fwd.Dst {
		dstSet[n.String()] = struct{}{}
	}
	if _, ok := dstSet["10.20.0.0/24"]; !ok {
		t.Fatalf("expected dev-fwd Dst to contain remote egress range 10.20.0.0/24, got %v", fwd.Dst)
	}
	if _, ok := rules["acl-dev-to-remote-egress#dev-fwd-reverse"]; ok {
		t.Fatalf("did not expect reverse rule for uni-directional policy, rules: %+v", rules)
	}
}

// TestGetEgressRulesForNode_RemoteEgressDeviceBiEmitsReverse verifies Bi-directional
// policies emit a -reverse companion so return traffic from the remote egress range
// to the relayed device is also allowed at targetnode's forward chain.
func TestGetEgressRulesForNode_RemoteEgressDeviceBiEmitsReverse(t *testing.T) {
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalGetTagMap := GetTagMapWithNodesByNetwork
	originalListExtClients := listNetworkExtClients
	originalGetNodeByID := getNodeByID
	t.Cleanup(func() {
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		GetTagMapWithNodesByNetwork = originalGetTagMap
		listNetworkExtClients = originalListExtClients
		getNodeByID = originalGetNodeByID
	})

	targetID := uuid.New()
	relayedID := uuid.New()
	remoteOwnerID := uuid.New()
	relayedNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      relayedID,
			Network: "netmaker",
			Address: net.IPNet{
				IP:   net.ParseIP("100.64.0.20"),
				Mask: net.CIDRMask(32, 32),
			},
		},
		Tags: map[models.TagID]struct{}{"marketing": {}},
	}
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:           targetID,
			Network:      "netmaker",
			RelayedNodes: []string{relayedID.String()},
		},
		Tags: map[models.TagID]struct{}{},
	}

	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return []schema.Egress{
			{
				ID:      "egress-local",
				Network: network,
				Status:  true,
				Range:   "10.50.0.0/24",
				Nodes:   datatypes.JSONMap{targetID.String(): json.Number("100")},
			},
			{
				ID:      "egress-remote",
				Network: network,
				Status:  true,
				Range:   "10.20.0.0/24",
				Nodes:   datatypes.JSONMap{remoteOwnerID.String(): json.Number("100")},
			},
		}, nil
	}
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "acl-dev-bi",
			Enabled:          true,
			AllowedDirection: models.TrafficDirectionBi,
			Proto:            models.ALL,
			Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "marketing"}},
			Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "egress-remote"}},
		}}
	}
	GetTagMapWithNodesByNetwork = func(netID schema.NetworkID, withStatic bool) map[models.TagID][]models.Node {
		return map[models.TagID][]models.Node{}
	}
	listNetworkExtClients = func(network string) ([]models.ExtClient, error) {
		return nil, nil
	}
	getNodeByID = func(id string) (models.Node, error) {
		if id == relayedID.String() {
			return relayedNode, nil
		}
		return models.Node{}, fmt.Errorf("not found")
	}

	rules := GetEgressRulesForNode(targetNode)
	rev, ok := rules["acl-dev-bi#dev-fwd-reverse"]
	if !ok {
		t.Fatalf("expected -reverse companion for Bi dev-fwd, rules: %+v", rules)
	}
	revSrcSet := make(map[string]struct{}, len(rev.IPList))
	for _, n := range rev.IPList {
		revSrcSet[n.String()] = struct{}{}
	}
	if _, ok := revSrcSet["10.20.0.0/24"]; !ok {
		t.Fatalf("expected reverse IPList to contain remote egress range, got %v", rev.IPList)
	}
	revDstSet := make(map[string]struct{}, len(rev.Dst))
	for _, n := range rev.Dst {
		revDstSet[n.String()] = struct{}{}
	}
	if _, ok := revDstSet["100.64.0.20/32"]; !ok {
		t.Fatalf("expected reverse Dst to contain relayed device /32, got %v", rev.Dst)
	}
}

// TestGetEgressRulesForNode_MixedSrcEmitsBothDeviceAndSiteToSiteRules verifies
// that when an acl.Src mixes EgressID + NodeID + NetmakerIPAclID (e.g. a
// "site + device" policy that lets both a mesh device and an egress-LAN IP
// reach a remote egress), the egress node emits BOTH:
//   - rules[acl.ID]         the device->egress rule (from the main loop),
//                           sourced from the mesh node's VPN address;
//   - rules[acl.ID#xs0]     the site-to-site rule (from
//                           getEgressAclRulesForTargetNode),
//                           sourced from the selected egress source IP / mesh
//                           net of the source-egress router.
//
// Before the egressSiteToSiteRuleKey fix, the site-to-site rule overwrote the
// main-loop rule under the same `acl.ID` key, so the device source (e.g. the
// MacBook's mesh IP) was silently dropped on the egress node and its traffic
// was blackholed at the firewall.
func TestGetEgressRulesForNode_MixedSrcEmitsBothDeviceAndSiteToSiteRules(t *testing.T) {
	originalGetEgressByID := getEgressByID
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalGetTagMap := GetTagMapWithNodesByNetwork
	originalListExtClients := listNetworkExtClients
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		GetTagMapWithNodesByNetwork = originalGetTagMap
		listNetworkExtClients = originalListExtClients
	})

	// targetID is the egress node hosting "dst-egress" (the "vr" egress in the
	// real-world report). macbookID is a regular netclient mesh device whose
	// VPN address must end up in the device-rule's IPList.
	targetID := uuid.New()
	srcEgressOwnerID := uuid.New()
	macbookID := uuid.New()
	macbookNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      macbookID,
			Network: "netmaker",
			Address: net.IPNet{
				IP:   net.ParseIP("100.64.0.50"),
				Mask: net.CIDRMask(32, 32),
			},
		},
	}
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
		},
		Tags: map[models.TagID]struct{}{},
	}

	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return []schema.Egress{
			{
				ID:      "src-egress",
				Network: network,
				Status:  true,
				Range:   "10.110.0.0/20",
				Nodes:   datatypes.JSONMap{srcEgressOwnerID.String(): json.Number("100")},
			},
			{
				ID:      "dst-egress",
				Network: network,
				Status:  true,
				Range:   "10.104.0.0/20",
				Nodes:   datatypes.JSONMap{targetID.String(): json.Number("100")},
			},
		}, nil
	}
	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "src-egress":
			return schema.Egress{
				ID:     "src-egress",
				Status: true,
				Range:  "10.110.0.0/20",
				Nodes:  datatypes.JSONMap{srcEgressOwnerID.String(): json.Number("100")},
			}, nil
		case "dst-egress":
			return schema.Egress{
				ID:     "dst-egress",
				Status: true,
				Range:  "10.104.0.0/20",
				Nodes:  datatypes.JSONMap{targetID.String(): json.Number("100")},
			}, nil
		}
		return schema.Egress{}, errors.New("not found")
	}
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "site-acl",
			Enabled:          true,
			AllowedDirection: models.TrafficDirectionBi,
			Proto:            models.ALL,
			Src: []models.AclPolicyTag{
				{ID: models.EgressID, Value: "src-egress"},
				{ID: models.NodeID, Value: macbookID.String()},
				{ID: models.NetmakerIPAclID, Value: "10.110.0.112/32"},
			},
			Dst: []models.AclPolicyTag{
				{ID: models.EgressID, Value: "dst-egress"},
				{ID: models.NetmakerIPAclID, Value: "10.104.0.16/32"},
			},
		}}
	}
	GetTagMapWithNodesByNetwork = func(netID schema.NetworkID, withStatic bool) map[models.TagID][]models.Node {
		return map[models.TagID][]models.Node{
			models.TagID(macbookID.String()): {macbookNode},
		}
	}
	listNetworkExtClients = func(network string) ([]models.ExtClient, error) { return nil, nil }

	rules := GetEgressRulesForNode(targetNode)

	// Device rule: from MacBook's mesh VPN IP to the selected egress IP.
	dev, ok := rules["site-acl"]
	if !ok {
		t.Fatalf("expected device rule keyed by acl.ID, got: %+v", rules)
	}
	devSrcs := map[string]struct{}{}
	for _, n := range dev.IPList {
		devSrcs[n.String()] = struct{}{}
	}
	if _, ok := devSrcs["100.64.0.50/32"]; !ok {
		t.Fatalf("expected device rule IPList to contain MacBook mesh IP 100.64.0.50/32, got %v", dev.IPList)
	}
	devDsts := map[string]struct{}{}
	for _, n := range dev.Dst {
		devDsts[n.String()] = struct{}{}
	}
	if _, ok := devDsts["10.104.0.16/32"]; !ok {
		t.Fatalf("expected device rule Dst to contain 10.104.0.16/32, got %v", dev.Dst)
	}

	// Site-to-site rule: from the selected source egress IP to the selected
	// dst egress IP. Lives under "#xs0" so it does not overwrite the device rule.
	s2s, ok := rules["site-acl#xs0"]
	if !ok {
		t.Fatalf("expected site-to-site rule keyed by acl.ID + \"#xs0\", got: %+v", rules)
	}
	s2sSrcs := map[string]struct{}{}
	for _, n := range s2s.IPList {
		s2sSrcs[n.String()] = struct{}{}
	}
	if _, ok := s2sSrcs["10.110.0.112/32"]; !ok {
		t.Fatalf("expected site-to-site IPList to contain selected src IP 10.110.0.112/32, got %v", s2s.IPList)
	}
	s2sDsts := map[string]struct{}{}
	for _, n := range s2s.Dst {
		s2sDsts[n.String()] = struct{}{}
	}
	if _, ok := s2sDsts["10.104.0.16/32"]; !ok {
		t.Fatalf("expected site-to-site Dst to contain selected dst IP 10.104.0.16/32, got %v", s2s.Dst)
	}

	// Reverse rules for Bi: device-reverse keyed by acl.ID + "-reverse",
	// site-to-site reverse keyed by acl.ID + "-reverse#xs0".
	if _, ok := rules["site-acl-reverse"]; !ok {
		t.Fatalf("expected device reverse rule keyed by acl.ID + \"-reverse\", got: %+v", rules)
	}
	if _, ok := rules["site-acl-reverse#xs0"]; !ok {
		t.Fatalf("expected site-to-site reverse rule keyed by acl.ID + \"-reverse#xs0\", got: %+v", rules)
	}
}

// TestGetEgressRulesForNode_UniMixedSrcMultiDstIPsOnDstSideNode is the exact-scenario
// regression for the user's `site-acl` (one-way) report:
//   - src has {EgressID: blr-eg} + {NodeID: macbook} + {NetmakerIPAclID: 10.110.0.112/32}
//   - dst has {EgressID: vr}     + {NetmakerIPAclID: 10.104.0.16/32} + {NetmakerIPAclID: 10.104.0.4/32}
//   - Direction is Uni
//
// targetnode = vr-owner (the "dst egress node"). The dst-side node must receive
// both the device-rule (acl.ID) covering the MacBook -> selected-dst-IPs path
// AND the site-to-site rules (acl.ID#xs0 / #xs1) covering each
// (selected-src-IP, selected-dst-IP) pair. Without the Uni-guard fix, the
// site-to-site pass bailed out on the dst-side node and the rule was missing
// for one-way traffic from the src egress LAN to the dst egress LAN.
func TestGetEgressRulesForNode_UniMixedSrcMultiDstIPsOnDstSideNode(t *testing.T) {
	originalGetEgressByID := getEgressByID
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalGetTagMap := GetTagMapWithNodesByNetwork
	originalListExtClients := listNetworkExtClients
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		GetTagMapWithNodesByNetwork = originalGetTagMap
		listNetworkExtClients = originalListExtClients
	})

	targetID := uuid.New()       // vr-owner
	srcEgressOwnerID := uuid.New() // blr-eg owner
	macbookID := uuid.New()
	macbookNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      macbookID,
			Network: "netmaker",
			Address: net.IPNet{
				IP:   net.ParseIP("100.64.0.50"),
				Mask: net.CIDRMask(32, 32),
			},
		},
	}
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
		},
		Tags: map[models.TagID]struct{}{},
	}

	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return []schema.Egress{
			{
				ID:      "blr-eg",
				Network: network,
				Status:  true,
				Range:   "10.110.0.0/20",
				Nodes:   datatypes.JSONMap{srcEgressOwnerID.String(): json.Number("100")},
			},
			{
				ID:      "vr",
				Network: network,
				Status:  true,
				Range:   "10.104.0.0/20",
				Nodes:   datatypes.JSONMap{targetID.String(): json.Number("100")},
			},
		}, nil
	}
	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "blr-eg":
			return schema.Egress{
				ID:     "blr-eg",
				Status: true,
				Range:  "10.110.0.0/20",
				Nodes:  datatypes.JSONMap{srcEgressOwnerID.String(): json.Number("100")},
			}, nil
		case "vr":
			return schema.Egress{
				ID:     "vr",
				Status: true,
				Range:  "10.104.0.0/20",
				Nodes:  datatypes.JSONMap{targetID.String(): json.Number("100")},
			}, nil
		}
		return schema.Egress{}, errors.New("not found")
	}
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "site-acl",
			Enabled:          true,
			AllowedDirection: models.TrafficDirectionUni,
			Proto:            models.ALL,
			Src: []models.AclPolicyTag{
				{ID: models.EgressID, Value: "blr-eg"},
				{ID: models.NodeID, Value: macbookID.String()},
				{ID: models.NetmakerIPAclID, Value: "10.110.0.112/32"},
			},
			Dst: []models.AclPolicyTag{
				{ID: models.EgressID, Value: "vr"},
				{ID: models.NetmakerIPAclID, Value: "10.104.0.16/32"},
				{ID: models.NetmakerIPAclID, Value: "10.104.0.4/32"},
			},
		}}
	}
	GetTagMapWithNodesByNetwork = func(netID schema.NetworkID, withStatic bool) map[models.TagID][]models.Node {
		return map[models.TagID][]models.Node{
			models.TagID(macbookID.String()): {macbookNode},
		}
	}
	listNetworkExtClients = func(network string) ([]models.ExtClient, error) { return nil, nil }

	rules := GetEgressRulesForNode(targetNode)

	// Two site-to-site rules, one per (selected-src-IP x selected-dst-IP) pair.
	wantPairs := map[string]string{
		"site-acl#xs0": "10.104.0.16/32",
		"site-acl#xs1": "10.104.0.4/32",
	}
	// crossSiteEgressIPNetPairs order is map-iteration sensitive, so we just
	// assert that BOTH pairs are present (under either #xs0 or #xs1).
	foundDsts := map[string]struct{}{}
	for k, r := range rules {
		if k != "site-acl#xs0" && k != "site-acl#xs1" {
			continue
		}
		// each rule must contain the selected src IP as the only IPList entry
		if len(r.IPList) != 1 || r.IPList[0].String() != "10.110.0.112/32" {
			t.Fatalf("%s: expected IPList=[10.110.0.112/32], got %v", k, r.IPList)
		}
		if len(r.Dst) != 1 {
			t.Fatalf("%s: expected single Dst, got %v", k, r.Dst)
		}
		foundDsts[r.Dst[0].String()] = struct{}{}
		if r.Direction != models.TrafficDirectionUni {
			t.Fatalf("%s: expected Uni direction, got %v", k, r.Direction)
		}
	}
	for _, dst := range wantPairs {
		if _, ok := foundDsts[dst]; !ok {
			t.Fatalf("missing site-to-site rule for dst %s, got rules: %+v", dst, rules)
		}
	}

	// Uni must NOT emit reverse legs.
	if _, ok := rules["site-acl-reverse#xs0"]; ok {
		t.Fatalf("did not expect reverse rule for uni-directional policy, rules: %+v", rules)
	}
	if _, ok := rules["site-acl-reverse#xs1"]; ok {
		t.Fatalf("did not expect reverse rule for uni-directional policy, rules: %+v", rules)
	}

	// The device-rule (MacBook mesh IP -> selected dst IPs) must also coexist
	// under acl.ID, independent of the site-to-site rules.
	dev, ok := rules["site-acl"]
	if !ok {
		t.Fatalf("expected device rule keyed by acl.ID for MacBook mesh source, got: %+v", rules)
	}
	devSrcs := map[string]struct{}{}
	for _, n := range dev.IPList {
		devSrcs[n.String()] = struct{}{}
	}
	if _, ok := devSrcs["100.64.0.50/32"]; !ok {
		t.Fatalf("expected device rule IPList to contain MacBook mesh IP 100.64.0.50/32, got %v", dev.IPList)
	}
}

// TestGetAclRulesForNode_UniSrcEgressMeshIPInDstSideRule is the
// peer-acl regression for the user's report ("on the dst egress node i still
// don't see rules for allowing other site egress range when one way traffic"
// / "src egress vpn node ... handshake itself doesn't occur"):
//
// site-acl is Uni, src has {EgressID: blr-eg, NodeID: macbook,
// NetmakerIPAclID: 10.110.0.112/32}, dst has {EgressID: vr,
// NetmakerIPAclID: 10.104.0.16/32, NetmakerIPAclID: 10.104.0.4/32}.
//
// targetnode = vr-owner. The dst-side node's GetAclRulesForNode rule must
// list BOTH the src-egress router's mesh IP (so its INPUT chain accepts the
// incoming wg traffic from the blr-eg router) AND the regular mesh device
// referenced as a NodeID (MacBook). Before the fix, ConvAclTagToValueMap
// dropped the EgressID's "shape", leaving "blr-eg" as a literal map key that
// no taggedNode lookup ever resolved, so the src-egress mesh router was
// silently absent from IPList and the wg handshake/L4 traffic was dropped.
func TestGetAclRulesForNode_UniSrcEgressMeshIPInDstSideRule(t *testing.T) {
	originalGetEgressByID := getEgressByID
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalGetTagMap := GetTagMapWithNodesByNetwork
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		GetTagMapWithNodesByNetwork = originalGetTagMap
	})

	targetID := uuid.New()         // vr-owner = dst egress node
	srcEgressOwnerID := uuid.New() // blr-eg owner = src egress node
	macbookID := uuid.New()
	macbookNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      macbookID,
			Network: "netmaker",
			Address: net.IPNet{
				IP:   net.ParseIP("100.64.0.50"),
				Mask: net.CIDRMask(32, 32),
			},
		},
	}
	srcEgressOwnerNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      srcEgressOwnerID,
			Network: "netmaker",
			Address: net.IPNet{
				IP:   net.ParseIP("100.64.0.10"),
				Mask: net.CIDRMask(32, 32),
			},
		},
	}
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
			Address: net.IPNet{
				IP:   net.ParseIP("100.64.0.20"),
				Mask: net.CIDRMask(32, 32),
			},
		},
		Tags: map[models.TagID]struct{}{},
	}

	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "blr-eg":
			return schema.Egress{
				ID:     "blr-eg",
				Status: true,
				Range:  "10.110.0.0/20",
				Nodes:  datatypes.JSONMap{srcEgressOwnerID.String(): json.Number("100")},
			}, nil
		case "vr":
			return schema.Egress{
				ID:     "vr",
				Status: true,
				Range:  "10.104.0.0/20",
				Nodes:  datatypes.JSONMap{targetID.String(): json.Number("100")},
			}, nil
		}
		return schema.Egress{}, errors.New("not found")
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) { return nil, nil }
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "site-acl",
			Enabled:          true,
			AllowedDirection: models.TrafficDirectionUni,
			Proto:            models.ALL,
			Src: []models.AclPolicyTag{
				{ID: models.EgressID, Value: "blr-eg"},
				{ID: models.NodeID, Value: macbookID.String()},
				{ID: models.NetmakerIPAclID, Value: "10.110.0.112/32"},
			},
			Dst: []models.AclPolicyTag{
				{ID: models.EgressID, Value: "vr"},
				{ID: models.NetmakerIPAclID, Value: "10.104.0.16/32"},
				{ID: models.NetmakerIPAclID, Value: "10.104.0.4/32"},
			},
		}}
	}
	// Both nodes are advertised under their NodeID keys so the existing
	// taggedNodes[NodeID] lookup resolves their mesh AddressIPNet4.
	GetTagMapWithNodesByNetwork = func(netID schema.NetworkID, withStatic bool) map[models.TagID][]models.Node {
		return map[models.TagID][]models.Node{
			models.TagID(macbookID.String()):        {macbookNode},
			models.TagID(srcEgressOwnerID.String()): {srcEgressOwnerNode},
		}
	}

	rules := GetAclRulesForNode(&targetNode)
	rule, ok := rules["site-acl"]
	if !ok {
		t.Fatalf("expected peer-acl rule keyed by acl.ID for Uni site-to-site policy on dst-side node, got rules: %+v", rules)
	}
	got := map[string]struct{}{}
	for _, n := range rule.IPList {
		got[n.String()] = struct{}{}
	}
	if _, ok := got["100.64.0.10/32"]; !ok {
		t.Fatalf("expected dst-side rule IPList to contain src-egress router mesh IP 100.64.0.10/32 (blr-eg owner), got %v", rule.IPList)
	}
	if _, ok := got["100.64.0.50/32"]; !ok {
		t.Fatalf("expected dst-side rule IPList to also contain mesh device NodeID source (MacBook 100.64.0.50/32), got %v", rule.IPList)
	}
	dstStrs := map[string]struct{}{}
	for _, d := range rule.Dst {
		dstStrs[d.String()] = struct{}{}
	}
	for _, want := range []string{"100.64.0.20/32", "10.104.0.16/32", "10.104.0.4/32"} {
		if _, ok := dstStrs[want]; !ok {
			t.Fatalf("expected dst-side rule Dst to contain %s, got %v", want, rule.Dst)
		}
	}
	if rule.Direction != models.TrafficDirectionUni {
		t.Fatalf("expected Uni direction on dst-side rule, got %v", rule.Direction)
	}
}

// TestGetAclRulesForNode_UniSrcEgressNoEgressDoesNotInflateSrcTags is a
// negative companion: the src-side EgressID expansion must be a NO-OP when
// the egress lookup fails or the egress is disabled, leaving the legacy
// behaviour untouched (no spurious src nodes leaking into IPList).
func TestGetAclRulesForNode_UniSrcEgressNoEgressDoesNotInflateSrcTags(t *testing.T) {
	originalGetEgressByID := getEgressByID
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalGetTagMap := GetTagMapWithNodesByNetwork
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		GetTagMapWithNodesByNetwork = originalGetTagMap
	})

	targetID := uuid.New()
	macbookID := uuid.New()
	macbookNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      macbookID,
			Network: "netmaker",
			Address: net.IPNet{
				IP:   net.ParseIP("100.64.0.50"),
				Mask: net.CIDRMask(32, 32),
			},
		},
	}
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
			Address: net.IPNet{
				IP:   net.ParseIP("100.64.0.20"),
				Mask: net.CIDRMask(32, 32),
			},
		},
		Tags: map[models.TagID]struct{}{},
	}
	getEgressByID = func(egressID string) (schema.Egress, error) {
		// "missing-eg" is not found; "vr" exists and is owned by the target.
		switch egressID {
		case "vr":
			return schema.Egress{
				ID:     "vr",
				Status: true,
				Range:  "10.104.0.0/20",
				Nodes:  datatypes.JSONMap{targetID.String(): json.Number("100")},
			}, nil
		}
		return schema.Egress{}, errors.New("not found")
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) { return nil, nil }
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "site-acl-missing-src-egress",
			Enabled:          true,
			AllowedDirection: models.TrafficDirectionUni,
			Proto:            models.ALL,
			Src: []models.AclPolicyTag{
				{ID: models.EgressID, Value: "missing-eg"},
				{ID: models.NodeID, Value: macbookID.String()},
			},
			Dst: []models.AclPolicyTag{
				{ID: models.EgressID, Value: "vr"},
			},
		}}
	}
	GetTagMapWithNodesByNetwork = func(netID schema.NetworkID, withStatic bool) map[models.TagID][]models.Node {
		return map[models.TagID][]models.Node{
			models.TagID(macbookID.String()): {macbookNode},
		}
	}

	rules := GetAclRulesForNode(&targetNode)
	rule, ok := rules["site-acl-missing-src-egress"]
	if !ok {
		t.Fatalf("expected acl rule, got: %+v", rules)
	}
	got := map[string]struct{}{}
	for _, n := range rule.IPList {
		got[n.String()] = struct{}{}
	}
	// Only the mesh NodeID entry should be present; missing-eg expands to nothing.
	if _, ok := got["100.64.0.50/32"]; !ok {
		t.Fatalf("expected MacBook 100.64.0.50/32 in IPList, got %v", rule.IPList)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly one IPList entry (MacBook); got %d: %v", len(got), rule.IPList)
	}
}

func TestMapEgressIPNetToVirtualNAT(t *testing.T) {
	_, realHost, _ := net.ParseCIDR("10.104.0.16/32")
	e := schema.Egress{
		Range:        "10.104.0.0/24",
		VirtualRange: "100.64.5.0/24",
		Nat:          true,
		Mode:         schema.VirtualNAT,
	}
	mapped, ok := MapEgressIPNetToVirtualNAT(*realHost, e)
	if !ok {
		t.Fatal("expected successful virtual NAT mapping")
	}
	if mapped.String() != "100.64.5.16/32" {
		t.Fatalf("expected 100.64.5.16/32, got %s", mapped.String())
	}
}

func TestNormalizeAndValidateAclEgressIPs_AcceptsVirtualRangeIP(t *testing.T) {
	originalGetEgressByID := getEgressByID
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
	})

	getEgressByID = func(egressID string) (schema.Egress, error) {
		return schema.Egress{
			ID:           egressID,
			Range:        "10.104.0.0/24",
			VirtualRange: "100.64.5.0/24",
			Nat:          true,
			Mode:         schema.VirtualNAT,
		}, nil
	}

	acl := models.Acl{
		Dst: []models.AclPolicyTag{
			{ID: models.EgressID, Value: "eg-1"},
			{ID: models.NetmakerIPAclID, Value: "100.64.5.16"},
		},
	}
	if err := NormalizeAndValidateAclEgressIPs(&acl); err != nil {
		t.Fatalf("expected virtual range ip to validate, got: %v", err)
	}
}

func TestComputeEgressDstsForAcl_VirtualNATRestrictedIP(t *testing.T) {
	ownerID := uuid.New().String()
	remoteID := uuid.New().String()
	eg := schema.Egress{
		ID:           "eg-vnat",
		Range:        "10.104.0.0/24",
		VirtualRange: "100.64.5.0/24",
		Nat:          true,
		Mode:         schema.VirtualNAT,
		Nodes:        datatypes.JSONMap{ownerID: json.Number("100")},
	}
	egByID := map[string]schema.Egress{"eg-vnat": eg}
	acl := models.Acl{
		Dst: []models.AclPolicyTag{
			{ID: models.EgressID, Value: "eg-vnat"},
			{ID: models.NetmakerIPAclID, Value: "10.104.0.16/32"},
		},
	}

	owner4, _ := computeEgressDstsForAcl(ownerID, acl, egByID)
	if len(owner4) != 1 || owner4[0].String() != "10.104.0.16/32" {
		t.Fatalf("owner should see real LAN ip, got %v", owner4)
	}

	remote4, _ := computeEgressDstsForAcl(remoteID, acl, egByID)
	if len(remote4) != 1 || remote4[0].String() != "100.64.5.16/32" {
		t.Fatalf("remote node should see virtual ip, got %v", remote4)
	}
}

func TestGetEgressRulesForNode_SiteToSiteVirtualNATRestrictedIP(t *testing.T) {
	originalGetEgressByID := getEgressByID
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalGetTagMap := GetTagMapWithNodesByNetwork
	originalListExtClients := listNetworkExtClients
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		GetTagMapWithNodesByNetwork = originalGetTagMap
		listNetworkExtClients = originalListExtClients
	})

	targetID := uuid.New()
	srcEgressOwnerID := uuid.New()
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
		},
		Tags: map[models.TagID]struct{}{},
	}

	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return []schema.Egress{
			{
				ID:           "src-egress",
				Network:      network,
				Status:       true,
				Range:        "10.110.0.0/24",
				VirtualRange: "100.64.10.0/24",
				Nat:          true,
				Mode:         schema.VirtualNAT,
				Nodes:        datatypes.JSONMap{srcEgressOwnerID.String(): json.Number("100")},
			},
			{
				ID:      "dst-egress",
				Network: network,
				Status:  true,
				Range:   "10.104.0.0/24",
				Nodes:   datatypes.JSONMap{targetID.String(): json.Number("100")},
			},
		}, nil
	}
	getEgressByID = func(egressID string) (schema.Egress, error) {
		for _, e := range []schema.Egress{
			{
				ID:           "src-egress",
				Status:       true,
				Range:        "10.110.0.0/24",
				VirtualRange: "100.64.10.0/24",
				Nat:          true,
				Mode:         schema.VirtualNAT,
				Nodes:        datatypes.JSONMap{srcEgressOwnerID.String(): json.Number("100")},
			},
			{
				ID:     "dst-egress",
				Status: true,
				Range:  "10.104.0.0/24",
				Nodes:  datatypes.JSONMap{targetID.String(): json.Number("100")},
			},
		} {
			if e.ID == egressID {
				return e, nil
			}
		}
		return schema.Egress{}, errors.New("not found")
	}
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "site-acl",
			Enabled:          true,
			AllowedDirection: models.TrafficDirectionBi,
			Proto:            models.ALL,
			Src: []models.AclPolicyTag{
				{ID: models.EgressID, Value: "src-egress"},
				{ID: models.NetmakerIPAclID, Value: "10.110.0.112/32"},
			},
			Dst: []models.AclPolicyTag{
				{ID: models.EgressID, Value: "dst-egress"},
				{ID: models.NetmakerIPAclID, Value: "10.104.0.16/32"},
			},
		}}
	}
	GetTagMapWithNodesByNetwork = func(netID schema.NetworkID, withStatic bool) map[models.TagID][]models.Node {
		return map[models.TagID][]models.Node{}
	}
	listNetworkExtClients = func(network string) ([]models.ExtClient, error) { return nil, nil }

	rules := GetEgressRulesForNode(targetNode)

	s2s, ok := rules["site-acl#xs0"]
	if !ok {
		t.Fatalf("expected site-to-site rule keyed by acl.ID + \"#xs0\", got: %+v", rules)
	}
	s2sSrcs := map[string]struct{}{}
	for _, n := range s2s.IPList {
		s2sSrcs[n.String()] = struct{}{}
	}
	if _, ok := s2sSrcs["100.64.10.112/32"]; !ok {
		t.Fatalf("expected site-to-site IPList to contain virtual src IP 100.64.10.112/32, got %v", s2s.IPList)
	}
	if _, ok := s2sSrcs["10.110.0.112/32"]; ok {
		t.Fatalf("site-to-site IPList should not contain real remote src on dst owner, got %v", s2s.IPList)
	}
	s2sDsts := map[string]struct{}{}
	for _, n := range s2s.Dst {
		s2sDsts[n.String()] = struct{}{}
	}
	if _, ok := s2sDsts["10.104.0.16/32"]; !ok {
		t.Fatalf("expected site-to-site Dst to contain local real dst IP 10.104.0.16/32, got %v", s2s.Dst)
	}
}

func TestGetEgressRulesForNode_SiteToSiteVirtualNATOnSrcOwner(t *testing.T) {
	originalGetEgressByID := getEgressByID
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalGetTagMap := GetTagMapWithNodesByNetwork
	originalListExtClients := listNetworkExtClients
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		GetTagMapWithNodesByNetwork = originalGetTagMap
		listNetworkExtClients = originalListExtClients
	})

	srcOwnerID := uuid.New()
	dstEgressOwnerID := uuid.New()
	srcOwnerNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      srcOwnerID,
			Network: "netmaker",
		},
		Tags: map[models.TagID]struct{}{},
	}

	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return []schema.Egress{
			{
				ID:      "src-egress",
				Network: network,
				Status:  true,
				Range:   "10.110.0.0/24",
				Nodes:   datatypes.JSONMap{srcOwnerID.String(): json.Number("100")},
			},
			{
				ID:           "dst-egress",
				Network:      network,
				Status:       true,
				Range:        "10.104.0.0/24",
				VirtualRange: "100.64.5.0/24",
				Nat:          true,
				Mode:         schema.VirtualNAT,
				Nodes:        datatypes.JSONMap{dstEgressOwnerID.String(): json.Number("100")},
			},
		}, nil
	}
	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "src-egress":
			return schema.Egress{
				ID:     "src-egress",
				Status: true,
				Range:  "10.110.0.0/24",
				Nodes:  datatypes.JSONMap{srcOwnerID.String(): json.Number("100")},
			}, nil
		case "dst-egress":
			return schema.Egress{
				ID:           "dst-egress",
				Status:       true,
				Range:        "10.104.0.0/24",
				VirtualRange: "100.64.5.0/24",
				Nat:          true,
				Mode:         schema.VirtualNAT,
				Nodes:        datatypes.JSONMap{dstEgressOwnerID.String(): json.Number("100")},
			}, nil
		}
		return schema.Egress{}, errors.New("not found")
	}
	getDevicePoliciesByNetwork = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "site-acl",
			Enabled:          true,
			AllowedDirection: models.TrafficDirectionBi,
			Proto:            models.ALL,
			Src: []models.AclPolicyTag{
				{ID: models.EgressID, Value: "src-egress"},
				{ID: models.NetmakerIPAclID, Value: "10.110.0.112/32"},
			},
			Dst: []models.AclPolicyTag{
				{ID: models.EgressID, Value: "dst-egress"},
				{ID: models.NetmakerIPAclID, Value: "10.104.0.16/32"},
			},
		}}
	}
	GetTagMapWithNodesByNetwork = func(netID schema.NetworkID, withStatic bool) map[models.TagID][]models.Node {
		return map[models.TagID][]models.Node{}
	}
	listNetworkExtClients = func(network string) ([]models.ExtClient, error) { return nil, nil }

	rules := GetEgressRulesForNode(srcOwnerNode)

	s2s, ok := rules["site-acl#xs0"]
	if !ok {
		t.Fatalf("expected site-to-site rule, got: %+v", rules)
	}
	s2sSrcs := map[string]struct{}{}
	for _, n := range s2s.IPList {
		s2sSrcs[n.String()] = struct{}{}
	}
	if _, ok := s2sSrcs["10.110.0.112/32"]; !ok {
		t.Fatalf("expected real local src 10.110.0.112/32, got %v", s2s.IPList)
	}
	s2sDsts := map[string]struct{}{}
	for _, n := range s2s.Dst {
		s2sDsts[n.String()] = struct{}{}
	}
	if _, ok := s2sDsts["100.64.5.16/32"]; !ok {
		t.Fatalf("expected virtual remote dst 100.64.5.16/32, got %v", s2s.Dst)
	}
	if _, ok := s2sDsts["10.104.0.16/32"]; ok {
		t.Fatalf("src owner should not use real remote dst in site-to-site rule, got %v", s2s.Dst)
	}
}

func TestSelectedEgressDstNetsForNode_VirtualNATRestrictedIP(t *testing.T) {
	ownerID := uuid.New().String()
	remoteID := uuid.New().String()
	eg := schema.Egress{
		Range:        "10.104.0.0/24",
		VirtualRange: "100.64.5.0/24",
		Nat:          true,
		Mode:         schema.VirtualNAT,
		Nodes:        datatypes.JSONMap{ownerID: json.Number("100")},
	}
	_, selected, _ := net.ParseCIDR("10.104.0.16/32")
	selected4 := []net.IPNet{*selected}

	owner4, _ := SelectedEgressDstNetsForNode(ownerID, eg, selected4, nil)
	if len(owner4) != 1 || owner4[0].String() != "10.104.0.16/32" {
		t.Fatalf("owner should see real LAN ip, got %v", owner4)
	}

	remote4, _ := SelectedEgressDstNetsForNode(remoteID, eg, selected4, nil)
	if len(remote4) != 1 || remote4[0].String() != "100.64.5.16/32" {
		t.Fatalf("remote node should see virtual ip, got %v", remote4)
	}
}
