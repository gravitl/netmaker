package logic

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

const scaleEgressCount = 200

// buildSameGatewaySiteEgresses returns n active SITE egresses all routed by the same node.
func buildSameGatewaySiteEgresses(nodeID string, n int) []schema.Egress {
	out := make([]schema.Egress, 0, n)
	for i := 0; i < n; i++ {
		// Distinct /24s: 10.<octet1>.<octet2>.0/24
		o1 := (i / 256) % 256
		o2 := i % 256
		id := fmt.Sprintf("eg-%d", i)
		out = append(out, schema.Egress{
			ID:     id,
			Status: true,
			Nat:    true,
			Range:  fmt.Sprintf("10.%d.%d.0/24", o1, o2),
			Nodes:  datatypes.JSONMap{nodeID: true},
		})
	}
	return out
}

func countXSRules(rules map[string]models.AclRule) int {
	n := 0
	for id := range rules {
		if strings.Contains(id, "#xs") {
			n++
		}
	}
	return n
}

// lanToLanRule reports whether a rule looks like SITE/LAN → SITE/LAN (no mesh 100.x source).
func lanToLanRule(r models.AclRule) bool {
	if len(r.IPList) == 0 || len(r.Dst) == 0 {
		return false
	}
	for _, s := range r.IPList {
		v4 := s.IP.To4()
		if v4 == nil {
			continue
		}
		// Mesh overlay commonly 100.64/10 or similar; treat 100.x as user/mesh source.
		if v4[0] == 100 {
			return false
		}
	}
	return true
}

func countLanToLanRules(rules map[string]models.AclRule) int {
	n := 0
	for _, r := range rules {
		if lanToLanRule(r) {
			n++
		}
	}
	return n
}

func TestGetEgressAclRules_200SameGateway_AllNodesDisabled_NoSiteToSite(t *testing.T) {
	origPolicies := getNetworkAccessDevicePolicies
	origByNet := getEgressByNetwork
	origByID := getEgressByID
	t.Cleanup(func() {
		getNetworkAccessDevicePolicies = origPolicies
		getEgressByNetwork = origByNet
		getEgressByID = origByID
	})

	nodeID := uuid.New()
	egresses := buildSameGatewaySiteEgresses(nodeID.String(), scaleEgressCount)
	byID := make(map[string]schema.Egress, len(egresses))
	for _, e := range egresses {
		byID[e.ID] = e
	}

	// Customer posture: All Nodes / All Gateways disabled; no other device policies.
	getNetworkAccessDevicePolicies = func(ctx context.Context, netID schema.NetworkID) []models.Acl {
		return []models.Acl{
			{
				ID:               "net.all-nodes",
				Name:             "All Nodes",
				Default:          true,
				Enabled:          false,
				RuleType:         models.DevicePolicy,
				AllowedDirection: models.TrafficDirectionBi,
				Proto:            models.ALL,
				Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}},
				Dst:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}},
			},
			{
				ID:               "net.all-gateways",
				Name:             "All Gateways",
				Default:          true,
				Enabled:          false,
				RuleType:         models.DevicePolicy,
				AllowedDirection: models.TrafficDirectionBi,
				Proto:            models.ALL,
				Src: []models.AclPolicyTag{{
					ID: models.NodeTagID, Value: "net.gateways",
				}},
				Dst: []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}},
			},
		}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) { return egresses, nil }
	getEgressByID = func(id string) (schema.Egress, error) {
		e, ok := byID[id]
		if !ok {
			return schema.Egress{}, fmt.Errorf("not found")
		}
		return e, nil
	}

	target := models.Node{CommonNode: models.CommonNode{ID: nodeID, Network: "net"}}
	rules := getEgressAclRulesForTargetNode(db.WithContext(context.Background()), target)
	xs := countXSRules(rules)

	t.Logf("assessment: %d SITE egresses on one node; All Nodes DISABLED", scaleEgressCount)
	t.Logf("assessment: getEgressAclRulesForTargetNode → total=%d #xs=%d", len(rules), xs)

	if xs != 0 {
		t.Fatalf("BUG: expected 0 #xs site-to-site rules with All Nodes disabled, got %d (total rules %d)", xs, len(rules))
	}
	if len(rules) != 0 {
		t.Fatalf("expected no site-to-site rules at all, got %d: %+v", len(rules), rules)
	}
}

func TestGetEgressAclRules_200SameGateway_AllNodesEnabled_SkipsSameGatewayPairs(t *testing.T) {
	origPolicies := getNetworkAccessDevicePolicies
	origByNet := getEgressByNetwork
	origByID := getEgressByID
	t.Cleanup(func() {
		getNetworkAccessDevicePolicies = origPolicies
		getEgressByNetwork = origByNet
		getEgressByID = origByID
	})

	nodeID := uuid.New()
	egresses := buildSameGatewaySiteEgresses(nodeID.String(), scaleEgressCount)
	byID := make(map[string]schema.Egress, len(egresses))
	for _, e := range egresses {
		byID[e.ID] = e
	}

	getNetworkAccessDevicePolicies = func(ctx context.Context, netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "net.all-nodes",
			Name:             "All Nodes",
			Default:          true,
			Enabled:          true,
			RuleType:         models.DevicePolicy,
			AllowedDirection: models.TrafficDirectionBi,
			Proto:            models.ALL,
			Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}},
			Dst:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}},
		}}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) { return egresses, nil }
	getEgressByID = func(id string) (schema.Egress, error) {
		e, ok := byID[id]
		if !ok {
			return schema.Egress{}, fmt.Errorf("not found")
		}
		return e, nil
	}

	target := models.Node{CommonNode: models.CommonNode{ID: nodeID, Network: "net"}}
	rules := getEgressAclRulesForTargetNode(db.WithContext(context.Background()), target)
	xs := countXSRules(rules)

	t.Logf("assessment: %d SITE egresses on one node; All Nodes ENABLED (Bi *↔*)", scaleEgressCount)
	t.Logf("assessment: getEgressAclRulesForTargetNode → total=%d #xs=%d (same-gateway pairs skipped)", len(rules), xs)

	if xs != 0 || len(rules) != 0 {
		t.Fatalf("expected 0 #xs rules for %d SITE ranges that all live on this gateway, got %d (total %d)", scaleEgressCount, xs, len(rules))
	}
}

func TestGetEgressAclRules_CrossGateway_KeepsOnlyRemotePairs(t *testing.T) {
	origPolicies := getNetworkAccessDevicePolicies
	origByNet := getEgressByNetwork
	origByID := getEgressByID
	t.Cleanup(func() {
		getNetworkAccessDevicePolicies = origPolicies
		getEgressByNetwork = origByNet
		getEgressByID = origByID
	})

	localID := uuid.New()
	remoteID := uuid.New()
	local := buildSameGatewaySiteEgresses(localID.String(), 2)
	remote := buildSameGatewaySiteEgresses(remoteID.String(), 2)
	for i := range remote {
		remote[i].ID = fmt.Sprintf("remote-%d", i)
		// Keep the ranges distinct from the local 10.0.0.0/24 and 10.0.1.0/24.
		remote[i].Range = fmt.Sprintf("10.1.%d.0/24", i)
	}
	egresses := append(local, remote...)
	byID := make(map[string]schema.Egress, len(egresses))
	for _, e := range egresses {
		byID[e.ID] = e
	}

	getNetworkAccessDevicePolicies = func(ctx context.Context, netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "net.all-nodes",
			Name:             "All Nodes",
			Default:          true,
			Enabled:          true,
			RuleType:         models.DevicePolicy,
			AllowedDirection: models.TrafficDirectionBi,
			Proto:            models.ALL,
			Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}},
			Dst:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}},
		}}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) { return egresses, nil }
	getEgressByID = func(id string) (schema.Egress, error) {
		e, ok := byID[id]
		if !ok {
			return schema.Egress{}, fmt.Errorf("not found")
		}
		return e, nil
	}

	target := models.Node{CommonNode: models.CommonNode{ID: localID, Network: "net"}}
	rules := getEgressAclRulesForTargetNode(db.WithContext(context.Background()), target)
	xs := countXSRules(rules)
	// 2 local × 2 remote × both ordered directions × forward and explicit reverse legs.
	// Same-gateway pairs (local↔local and remote↔remote) are omitted.
	want := 2 * 2 * 2 * 2
	t.Logf("assessment: 2 local + 2 remote SITE ranges → #xs=%d (expected %d)", xs, want)
	if xs != want {
		t.Fatalf("expected %d cross-gateway #xs rules, got %d: %+v", want, xs, rules)
	}
	for id, r := range rules {
		if len(r.IPList) != 1 || len(r.Dst) != 1 {
			t.Fatalf("%s: expected a single src and dst, got %v -> %v", id, r.IPList, r.Dst)
		}
		srcLocal := strings.HasPrefix(r.IPList[0].String(), "10.0.")
		dstLocal := strings.HasPrefix(r.Dst[0].String(), "10.0.")
		if srcLocal == dstLocal {
			t.Fatalf("%s: same-gateway pair leaked: %s -> %s", id, r.IPList[0], r.Dst[0])
		}
	}
}

func TestGetEgressAclRules_TagRoutedSameGateway_SkipsPairs(t *testing.T) {
	origPolicies := getNetworkAccessDevicePolicies
	origByNet := getEgressByNetwork
	origByID := getEgressByID
	origNodes := listNetworkNodes
	t.Cleanup(func() {
		getNetworkAccessDevicePolicies = origPolicies
		getEgressByNetwork = origByNet
		getEgressByID = origByID
		listNetworkNodes = origNodes
	})

	nodeID := uuid.New()
	const tag = "site-routers"
	egresses := buildSameGatewaySiteEgresses(nodeID.String(), 4)
	for i := range egresses {
		egresses[i].Nodes = datatypes.JSONMap{}
		egresses[i].Tags = datatypes.JSONMap{tag: 256}
	}
	byID := make(map[string]schema.Egress, len(egresses))
	for _, e := range egresses {
		byID[e.ID] = e
	}
	target := models.Node{
		CommonNode: models.CommonNode{ID: nodeID, Network: "net"},
		Tags:       map[models.TagID]struct{}{models.TagID(tag): {}},
	}
	getNetworkAccessDevicePolicies = func(ctx context.Context, netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "net.all-nodes",
			Enabled:          true,
			Default:          true,
			RuleType:         models.DevicePolicy,
			AllowedDirection: models.TrafficDirectionBi,
			Proto:            models.ALL,
			Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}},
			Dst:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}},
		}}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) { return egresses, nil }
	getEgressByID = func(id string) (schema.Egress, error) {
		e, ok := byID[id]
		if !ok {
			return schema.Egress{}, fmt.Errorf("not found")
		}
		return e, nil
	}
	listNetworkNodes = func(ctx context.Context, network string) ([]models.Node, error) {
		return []models.Node{target}, nil
	}

	rules := getEgressAclRulesForTargetNode(db.WithContext(context.Background()), target)
	xs := countXSRules(rules)
	t.Logf("assessment: %d tag-routed SITE ranges on one node → #xs=%d", len(egresses), xs)
	if xs != 0 {
		t.Fatalf("expected 0 #xs when every tag-routed range lives only on this node, got %d", xs)
	}
}

func TestGetEgressAclRules_TagRoutedCrossGateway_KeepsRemotePairs(t *testing.T) {
	origPolicies := getNetworkAccessDevicePolicies
	origByNet := getEgressByNetwork
	origByID := getEgressByID
	origNodes := listNetworkNodes
	t.Cleanup(func() {
		getNetworkAccessDevicePolicies = origPolicies
		getEgressByNetwork = origByNet
		getEgressByID = origByID
		listNetworkNodes = origNodes
	})

	localID := uuid.New()
	remoteID := uuid.New()
	local := buildSameGatewaySiteEgresses(localID.String(), 2)
	remote := buildSameGatewaySiteEgresses(remoteID.String(), 2)
	for i := range local {
		local[i].Nodes = datatypes.JSONMap{}
		local[i].Tags = datatypes.JSONMap{"local-sites": 256}
	}
	for i := range remote {
		remote[i].ID = fmt.Sprintf("remote-%d", i)
		remote[i].Range = fmt.Sprintf("10.1.%d.0/24", i)
		remote[i].Nodes = datatypes.JSONMap{}
		remote[i].Tags = datatypes.JSONMap{"remote-sites": 256}
	}
	egresses := append(local, remote...)
	byID := make(map[string]schema.Egress, len(egresses))
	for _, e := range egresses {
		byID[e.ID] = e
	}
	localNode := models.Node{
		CommonNode: models.CommonNode{ID: localID, Network: "net"},
		Tags:       map[models.TagID]struct{}{"local-sites": {}},
	}
	remoteNode := models.Node{
		CommonNode: models.CommonNode{ID: remoteID, Network: "net"},
		Tags:       map[models.TagID]struct{}{"remote-sites": {}},
	}
	getNetworkAccessDevicePolicies = func(ctx context.Context, netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "net.all-nodes",
			Enabled:          true,
			Default:          true,
			RuleType:         models.DevicePolicy,
			AllowedDirection: models.TrafficDirectionBi,
			Proto:            models.ALL,
			Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}},
			Dst:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}},
		}}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) { return egresses, nil }
	getEgressByID = func(id string) (schema.Egress, error) {
		e, ok := byID[id]
		if !ok {
			return schema.Egress{}, fmt.Errorf("not found")
		}
		return e, nil
	}
	listNetworkNodes = func(ctx context.Context, network string) ([]models.Node, error) {
		return []models.Node{localNode, remoteNode}, nil
	}

	rules := getEgressAclRulesForTargetNode(db.WithContext(context.Background()), localNode)
	xs := countXSRules(rules)
	want := 2 * 2 * 2 * 2
	t.Logf("assessment: tag-routed 2 local + 2 remote → #xs=%d (expected %d)", xs, want)
	if xs != want {
		t.Fatalf("expected %d cross-gateway #xs rules for tag-routed ranges, got %d: %+v", want, xs, rules)
	}
}

func TestGetEgressRulesForNode_200SameGateway_OnlyUserPolicies_NoLanToLan(t *testing.T) {
	origPolicies := getNetworkAccessDevicePolicies
	origByNet := getEgressByNetwork
	origByID := getEgressByID
	origTags := GetTagMapWithNodesByNetwork
	origUser := GetEgressUserRulesForNode
	origUserDev := GetUserDeviceEgressRulesForNode
	t.Cleanup(func() {
		getNetworkAccessDevicePolicies = origPolicies
		getEgressByNetwork = origByNet
		getEgressByID = origByID
		GetTagMapWithNodesByNetwork = origTags
		GetEgressUserRulesForNode = origUser
		GetUserDeviceEgressRulesForNode = origUserDev
	})

	nodeID := uuid.New()
	egresses := buildSameGatewaySiteEgresses(nodeID.String(), scaleEgressCount)
	byID := make(map[string]schema.Egress, len(egresses))
	for _, e := range egresses {
		byID[e.ID] = e
	}

	GetTagMapWithNodesByNetwork = func(ctx context.Context, networkID schema.NetworkID, withStaticNodes bool) map[models.TagID][]models.Node {
		return map[models.TagID][]models.Node{models.TagID("*"): {}}
	}
	getNetworkAccessDevicePolicies = func(ctx context.Context, netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:       "net.all-nodes",
			Enabled:  false,
			Default:  true,
			RuleType: models.DevicePolicy,
			Src:      []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}},
			Dst:      []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}},
		}}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) { return egresses, nil }
	getEgressByID = func(id string) (schema.Egress, error) {
		e, ok := byID[id]
		if !ok {
			return schema.Egress{}, fmt.Errorf("not found")
		}
		return e, nil
	}

	// Simulate ~one user RAC + one user-device rule per egress (customer-like, not full 140 policies).
	const usersPerEgress = 2
	GetEgressUserRulesForNode = func(ctx context.Context, targetnode *models.Node, rules map[string]models.AclRule) map[string]models.AclRule {
		for i, e := range egresses {
			_, cidr, err := net.ParseCIDR(e.Range)
			if err != nil {
				continue
			}
			id := fmt.Sprintf("user-pol-%d", i)
			rules[id] = models.AclRule{
				ID:      id,
				Allowed: true,
				IPList: []net.IPNet{{
					IP:   net.ParseIP("100.113.229.10"),
					Mask: net.CIDRMask(32, 32),
				}},
				Dst: []net.IPNet{*cidr},
			}
		}
		return rules
	}
	GetUserDeviceEgressRulesForNode = func(ctx context.Context, targetnode *models.Node, networkNodes []models.Node, rules map[string]models.AclRule) map[string]models.AclRule {
		for i, e := range egresses {
			_, cidr, err := net.ParseCIDR(e.Range)
			if err != nil {
				continue
			}
			id := fmt.Sprintf("user-pol-%d#user-device", i)
			rules[id] = models.AclRule{
				ID:      id,
				Allowed: true,
				IPList: []net.IPNet{{
					IP:   net.ParseIP("100.113.229.4"),
					Mask: net.CIDRMask(32, 32),
				}},
				Dst: []net.IPNet{*cidr},
			}
		}
		return rules
	}

	target := models.Node{
		CommonNode: models.CommonNode{ID: nodeID, Network: "net"},
		EgressDetails: models.EgressDetails{
			IsEgressGateway: true,
		},
	}
	rules := GetEgressRulesForNode(db.WithContext(context.Background()), target)
	xs := countXSRules(rules)
	lanLan := countLanToLanRules(rules)
	wantUser := scaleEgressCount * usersPerEgress

	t.Logf("assessment: GetEgressRulesForNode with All Nodes OFF + user→egress only")
	t.Logf("assessment: %d SITE on one GW → total=%d #xs=%d lan↔lan=%d (expected user rules≈%d)",
		scaleEgressCount, len(rules), xs, lanLan, wantUser)

	if xs != 0 {
		t.Fatalf("BUG: GetEgressRulesForNode emitted %d #xs rules with All Nodes disabled", xs)
	}
	if lanLan != 0 {
		t.Fatalf("BUG: GetEgressRulesForNode emitted %d lan↔lan rules with only user→egress policies", lanLan)
	}
	if len(rules) != wantUser {
		t.Fatalf("expected %d user→egress rules, got %d", wantUser, len(rules))
	}
}

func TestGetEgressRulesForNode_200SameGateway_AllNodesOn_RuleCountAssessment(t *testing.T) {
	origPolicies := getNetworkAccessDevicePolicies
	origByNet := getEgressByNetwork
	origByID := getEgressByID
	origTags := GetTagMapWithNodesByNetwork
	origUser := GetEgressUserRulesForNode
	origUserDev := GetUserDeviceEgressRulesForNode
	t.Cleanup(func() {
		getNetworkAccessDevicePolicies = origPolicies
		getEgressByNetwork = origByNet
		getEgressByID = origByID
		GetTagMapWithNodesByNetwork = origTags
		GetEgressUserRulesForNode = origUser
		GetUserDeviceEgressRulesForNode = origUserDev
	})

	nodeID := uuid.New()
	egresses := buildSameGatewaySiteEgresses(nodeID.String(), scaleEgressCount)
	byID := make(map[string]schema.Egress, len(egresses))
	for _, e := range egresses {
		byID[e.ID] = e
	}

	GetTagMapWithNodesByNetwork = func(ctx context.Context, networkID schema.NetworkID, withStaticNodes bool) map[models.TagID][]models.Node {
		// One infra peer so All Nodes src=* enumerates something in the main loop.
		peer := models.Node{
			CommonNode: models.CommonNode{
				ID:      uuid.New(),
				Network: "net",
				Address: net.IPNet{IP: net.ParseIP("100.64.0.2"), Mask: net.CIDRMask(32, 32)},
			},
		}
		return map[models.TagID][]models.Node{models.TagID("*"): {peer}}
	}
	getNetworkAccessDevicePolicies = func(ctx context.Context, netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "net.all-nodes",
			Enabled:          true,
			Default:          true,
			RuleType:         models.DevicePolicy,
			AllowedDirection: models.TrafficDirectionBi,
			Proto:            models.ALL,
			Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}},
			Dst:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}},
		}}
	}
	getEgressByNetwork = func(network string) ([]schema.Egress, error) { return egresses, nil }
	getEgressByID = func(id string) (schema.Egress, error) {
		e, ok := byID[id]
		if !ok {
			return schema.Egress{}, fmt.Errorf("not found")
		}
		return e, nil
	}
	GetEgressUserRulesForNode = func(ctx context.Context, targetnode *models.Node, rules map[string]models.AclRule) map[string]models.AclRule {
		return rules
	}
	GetUserDeviceEgressRulesForNode = func(ctx context.Context, targetnode *models.Node, networkNodes []models.Node, rules map[string]models.AclRule) map[string]models.AclRule {
		return rules
	}

	target := models.Node{
		CommonNode: models.CommonNode{ID: nodeID, Network: "net"},
		EgressDetails: models.EgressDetails{
			IsEgressGateway: true,
		},
	}
	rules := GetEgressRulesForNode(db.WithContext(context.Background()), target)
	xs := countXSRules(rules)

	t.Logf("assessment: GetEgressRulesForNode with All Nodes ON, %d SITE on one GW", scaleEgressCount)
	t.Logf("assessment: total=%d #xs=%d (same-gateway pairs skipped)", len(rules), xs)

	if xs != 0 {
		t.Fatalf("expected 0 #xs from All Nodes when all %d SITE ranges live on this gateway, got %d", scaleEgressCount, xs)
	}
}

func TestDedupeEgressAclRules_DropsOverlappingPairsKeepsReverse(t *testing.T) {
	_, lan, _ := net.ParseCIDR("10.104.0.0/20")
	_, a, _ := net.ParseCIDR("100.121.42.1/32")
	_, b, _ := net.ParseCIDR("100.121.42.6/32")
	rules := map[string]models.AclRule{
		"all-nodes": {
			ID:              "all-nodes",
			AllowedProtocol: models.ALL,
			Direction:       models.TrafficDirectionBi,
			Allowed:         true,
			IPList:          []net.IPNet{*a, *b},
			Dst:             []net.IPNet{*lan},
		},
		"narrow": {
			ID:              "narrow",
			AllowedProtocol: models.ALL,
			Direction:       models.TrafficDirectionBi,
			Allowed:         true,
			IPList:          []net.IPNet{*a},
			Dst:             []net.IPNet{*lan},
		},
		"all-nodes-reverse": {
			ID:              "all-nodes-reverse",
			AllowedProtocol: models.ALL,
			Direction:       models.TrafficDirectionBi,
			Allowed:         true,
			IPList:          []net.IPNet{*lan},
			Dst:             []net.IPNet{*a, *b},
		},
		"narrow-reverse": {
			ID:              "narrow-reverse",
			AllowedProtocol: models.ALL,
			Direction:       models.TrafficDirectionBi,
			Allowed:         true,
			IPList:          []net.IPNet{*lan},
			Dst:             []net.IPNet{*a},
		},
		"anywhere": {
			ID:              "anywhere",
			AllowedProtocol: models.ALL,
			Direction:       models.TrafficDirectionBi,
			Allowed:         true,
			IPList:          []net.IPNet{*lan},
		},
		"anywhere-dup": {
			ID:              "anywhere-dup",
			AllowedProtocol: models.ALL,
			Direction:       models.TrafficDirectionBi,
			Allowed:         true,
			IPList:          []net.IPNet{*lan},
		},
	}

	out := dedupeEgressAclRules(rules)
	got := map[string]int{}
	for _, r := range out {
		dsts := r.Dst
		if len(dsts) == 0 {
			dsts = []net.IPNet{{}}
		}
		for _, src := range r.IPList {
			for _, dst := range dsts {
				dk := "*"
				if len(r.Dst) > 0 {
					dk = dst.String()
				}
				got[src.String()+">"+dk]++
			}
		}
	}
	want := map[string]int{
		"100.121.42.1/32>10.104.0.0/20": 1,
		"100.121.42.6/32>10.104.0.0/20": 1,
		"10.104.0.0/20>100.121.42.1/32": 1,
		"10.104.0.0/20>100.121.42.6/32": 1,
		"10.104.0.0/20>*":               1,
	}
	if len(got) != len(want) {
		t.Fatalf("pairs = %v, want %v", got, want)
	}
	for k, n := range want {
		if got[k] != n {
			t.Fatalf("pair %s count=%d, want %d (all %v)", k, got[k], n, got)
		}
	}
}
