package logic

import (
	"encoding/json"
	"errors"
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

func TestIsEgressToEgressPolicyForTarget_UniDirection_IgnoresDstOnlyRoutingNode(t *testing.T) {
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

	if isEgressToEgressPolicyForTarget(policy, targetNode) {
		t.Fatal("expected uni-directional policy to be ignored when target node only routes destination egress")
	}
}

func TestGetEgressAclRulesForTargetNode_UsesAclIDKeyAndSideLocalSelectedIPs(t *testing.T) {
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
	rule, ok := rules["acl-1"]
	if !ok {
		t.Fatal("expected rule keyed by ACL ID")
	}
	rev, okRev := rules["acl-1-reverse"]
	if !okRev {
		t.Fatal("expected bidirectional policy to add reverse rule acl-1-reverse")
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

func TestGetEgressAclRulesForTargetNode_UniDirectionMismatchIgnored(t *testing.T) {
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
			return schema.Egress{ID: egressID, Status: true, Range: "10.10.0.0/24", Nodes: datatypes.JSONMap{uuid.New().String(): true}}, nil
		case "dst-egress":
			return schema.Egress{ID: egressID, Status: true, Range: "10.20.0.0/24", Nodes: datatypes.JSONMap{targetID.String(): true}}, nil
		default:
			return schema.Egress{}, errors.New("not found")
		}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) { return nil, nil }

	rules := getEgressAclRulesForTargetNode(targetNode)
	if len(rules) != 0 {
		t.Fatalf("expected no rules for uni-direction mismatch, got %d", len(rules))
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

func TestIsEgressRoutingPolicyAllowedForNodes_UniReverseDenied(t *testing.T) {
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

	if IsEgressRoutingPolicyAllowedForNodes(policy, node, peer) {
		t.Fatal("expected uni-direction policy to deny reverse-only routing participation")
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
	fwd, ok := rules["acl-nat"]
	if !ok {
		t.Fatal("expected forward rule acl-nat")
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
