package logic

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func userDeviceNode(owner, addr string) models.Node {
	return models.Node{
		CommonNode: models.CommonNode{
			ID:      uuid.New(),
			Network: "netmaker",
			Address: net.IPNet{IP: net.ParseIP(addr), Mask: net.CIDRMask(32, 32)},
		},
		OwnerID: owner,
	}
}

func TestUserDeviceSrcIPsForPolicy_MatchesUserAndGroupSources(t *testing.T) {
	devices := []models.Node{
		userDeviceNode("alice", "100.64.0.9"),
		userDeviceNode("bob", "100.64.0.10"),
		userDeviceNode("carol", "100.64.0.11"),
	}
	userGrpMap := map[schema.UserGroupID]map[string]struct{}{
		"netmaker-users": {"carol": {}},
	}
	policy := models.Acl{
		Src: []models.AclPolicyTag{
			{ID: models.UserAclID, Value: "alice"},
			{ID: models.UserGroupAclID, Value: "netmaker-users"},
		},
	}

	src4, src6 := userDeviceSrcIPsForPolicy(policy, devices, userGrpMap)
	if len(src6) != 0 {
		t.Fatalf("expected no v6 sources, got %v", src6)
	}
	got := make(map[string]struct{}, len(src4))
	for _, n := range src4 {
		got[n.String()] = struct{}{}
	}
	if len(got) != 2 {
		t.Fatalf("expected alice and carol devices only, got %v", src4)
	}
	for _, want := range []string{"100.64.0.9/32", "100.64.0.11/32"} {
		if _, ok := got[want]; !ok {
			t.Fatalf("expected %s in sources, got %v", want, src4)
		}
	}
}

// TestUserDeviceEgressRule_IPRestrictedPolicyYieldsSelectedIPOnly is the regression
// for the reported bug: a user policy granting the "sig" egress but restricted to
// 10.104.0.1/32 must produce exactly that destination with the device /32 as the
// source, never the whole 10.104.0.0/20.
func TestUserDeviceEgressRule_IPRestrictedPolicyYieldsSelectedIPOnly(t *testing.T) {
	routingNodeID := uuid.New()
	routingNode := models.Node{
		CommonNode: models.CommonNode{ID: routingNodeID, Network: "netmaker"},
	}
	device := userDeviceNode("alice", "100.64.0.9")
	egs := []schema.Egress{{
		ID:      "sig",
		Network: "netmaker",
		Status:  true,
		Range:   "10.104.0.0/20",
		Nodes:   datatypes.JSONMap{routingNodeID.String(): json.Number("100")},
	}}
	policy := models.Acl{
		ID:               "alice-to-sig",
		Enabled:          true,
		RuleType:         models.UserPolicy,
		AllowedDirection: models.TrafficDirectionUni,
		Proto:            models.ALL,
		Src:              []models.AclPolicyTag{{ID: models.UserAclID, Value: "alice"}},
		Dst: []models.AclPolicyTag{
			{ID: models.EgressID, Value: "sig"},
			{ID: models.NetmakerIPAclID, Value: "10.104.0.1/32"},
		},
	}

	src4, src6 := userDeviceSrcIPsForPolicy(policy, []models.Node{device}, nil)
	var dst4, dst6 []net.IPNet
	appendUserDevicePolicyEgressDsts(context.TODO(), &routingNode, policy, egs, &dst4, &dst6)

	rules := make(map[string]models.AclRule)
	addUserDeviceAclRule(rules, policy, src4, src6, dst4, dst6)

	rule, ok := rules["alice-to-sig"+userDeviceRuleSuffix]
	if !ok {
		t.Fatalf("expected a user-device rule, got: %+v", rules)
	}
	if len(rule.IPList) != 1 || rule.IPList[0].String() != "100.64.0.9/32" {
		t.Fatalf("expected the device /32 as the only source, got %v", rule.IPList)
	}
	if len(rule.Dst) != 1 || rule.Dst[0].String() != "10.104.0.1/32" {
		t.Fatalf("expected only the selected egress IP as destination, got %v", rule.Dst)
	}
}

func TestUserDeviceEgressRule_WithoutSelectedIPsUsesEgressRange(t *testing.T) {
	routingNodeID := uuid.New()
	routingNode := models.Node{
		CommonNode: models.CommonNode{ID: routingNodeID, Network: "netmaker"},
	}
	egs := []schema.Egress{{
		ID:      "sig",
		Network: "netmaker",
		Status:  true,
		Range:   "10.104.0.0/20",
		Nodes:   datatypes.JSONMap{routingNodeID.String(): json.Number("100")},
	}}
	policy := models.Acl{
		ID:       "alice-to-sig",
		Enabled:  true,
		RuleType: models.UserPolicy,
		Src:      []models.AclPolicyTag{{ID: models.UserAclID, Value: "alice"}},
		Dst:      []models.AclPolicyTag{{ID: models.EgressID, Value: "sig"}},
	}

	var dst4, dst6 []net.IPNet
	appendUserDevicePolicyEgressDsts(context.TODO(), &routingNode, policy, egs, &dst4, &dst6)
	if len(dst4) != 1 || dst4[0].String() != "10.104.0.0/20" {
		t.Fatalf("expected the full egress range without IP restrictions, got %v", dst4)
	}
	if len(dst6) != 0 {
		t.Fatalf("expected no v6 destinations, got %v", dst6)
	}
}

// TestUserDeviceEgressRule_TagAttachedRoutingNode covers egresses whose routing
// nodes are attached by tag: the API leaves Nodes empty in that case, so the
// egress must still be matched through Tags.
func TestUserDeviceEgressRule_TagAttachedRoutingNode(t *testing.T) {
	routingNode := models.Node{
		CommonNode: models.CommonNode{ID: uuid.New(), Network: "netmaker"},
		Tags:       map[models.TagID]struct{}{"routers": {}},
	}
	egs := []schema.Egress{{
		ID:           "sig",
		Network:      "netmaker",
		Status:       true,
		Range:        "10.104.0.0/20",
		VirtualRange: "100.64.0.0/20",
		Nodes:        datatypes.JSONMap{},
		Tags:         datatypes.JSONMap{"routers": json.Number("100")},
	}}
	policy := models.Acl{
		ID:       "alice-to-sig",
		Enabled:  true,
		RuleType: models.UserPolicy,
		Src:      []models.AclPolicyTag{{ID: models.UserAclID, Value: "alice"}},
		Dst:      []models.AclPolicyTag{{ID: models.EgressID, Value: "sig"}},
	}

	var dst4, dst6 []net.IPNet
	appendUserDevicePolicyEgressDsts(context.TODO(), &routingNode, policy, egs, &dst4, &dst6)
	if len(dst4) != 1 || dst4[0].String() != "10.104.0.0/20" {
		t.Fatalf("expected the real range for a tag-attached routing node, got %v", dst4)
	}

	other := models.Node{CommonNode: models.CommonNode{ID: uuid.New(), Network: "netmaker"}}
	dst4, dst6 = nil, nil
	appendUserDevicePolicyEgressDsts(context.TODO(), &other, policy, egs, &dst4, &dst6)
	if len(dst4) != 1 || dst4[0].String() != "100.64.0.0/20" {
		t.Fatalf("expected the virtual range for a non-routing node, got %v", dst4)
	}
	if len(dst6) != 0 {
		t.Fatalf("expected no v6 destinations, got %v", dst6)
	}
}

// TestUserDeviceEgressRule_InternetOnlyOnExitRouter ensures an internet-exit
// user policy does not emit 0.0.0.0/0 on unrelated site-egress gateways (that
// would appear as iptables "anywhere" and bypass site IP restrictions).
func TestUserDeviceEgressRule_InternetOnlyOnExitRouter(t *testing.T) {
	exitRouterID := uuid.New()
	siteRouterID := uuid.New()
	exitRouter := models.Node{
		CommonNode: models.CommonNode{ID: exitRouterID, Network: "netmaker"},
	}
	siteRouter := models.Node{
		CommonNode: models.CommonNode{ID: siteRouterID, Network: "netmaker"},
	}
	egs := []schema.Egress{
		{
			ID:      "exit2",
			Network: "netmaker",
			Status:  true,
			Type:    schema.EgressTypeInternet,
			Range:   "*",
			Nodes:   datatypes.JSONMap{exitRouterID.String(): json.Number("100")},
		},
		{
			ID:      "sig",
			Network: "netmaker",
			Status:  true,
			Range:   "10.104.0.0/20",
			Nodes:   datatypes.JSONMap{siteRouterID.String(): json.Number("100")},
		},
	}
	policy := models.Acl{
		ID:       "alice-to-exit",
		Enabled:  true,
		RuleType: models.UserPolicy,
		Src:      []models.AclPolicyTag{{ID: models.UserAclID, Value: "alice"}},
		Dst:      []models.AclPolicyTag{{ID: models.EgressID, Value: "exit2"}},
	}

	var dst4, dst6 []net.IPNet
	appendUserDevicePolicyEgressDsts(context.TODO(), &exitRouter, policy, egs, &dst4, &dst6)
	if len(dst4) == 0 {
		t.Fatal("expected default-route destinations on the internet exit router")
	}
	foundDefault := false
	for _, n := range dst4 {
		if ones, bits := n.Mask.Size(); ones == 0 && bits == 32 {
			foundDefault = true
			break
		}
	}
	if !foundDefault {
		t.Fatalf("expected 0.0.0.0/0 on the exit router, got %v", dst4)
	}

	dst4, dst6 = nil, nil
	appendUserDevicePolicyEgressDsts(context.TODO(), &siteRouter, policy, egs, &dst4, &dst6)
	if len(dst4) != 0 || len(dst6) != 0 {
		t.Fatalf("internet policy must not install anywhere-allows on a site egress gateway, got %v %v", dst4, dst6)
	}
}

func TestUserDeviceEgressRule_UnrelatedEgressIsNotADestination(t *testing.T) {
	routingNodeID := uuid.New()
	routingNode := models.Node{
		CommonNode: models.CommonNode{ID: routingNodeID, Network: "netmaker"},
	}
	egs := []schema.Egress{{
		ID:      "other",
		Network: "netmaker",
		Status:  true,
		Range:   "10.200.0.0/20",
		Nodes:   datatypes.JSONMap{routingNodeID.String(): json.Number("100")},
	}}
	policy := models.Acl{
		ID:       "alice-to-sig",
		Enabled:  true,
		RuleType: models.UserPolicy,
		Src:      []models.AclPolicyTag{{ID: models.UserAclID, Value: "alice"}},
		Dst:      []models.AclPolicyTag{{ID: models.EgressID, Value: "sig"}},
	}

	var dst4, dst6 []net.IPNet
	appendUserDevicePolicyEgressDsts(context.TODO(), &routingNode, policy, egs, &dst4, &dst6)
	if len(dst4) != 0 || len(dst6) != 0 {
		t.Fatalf("expected no destinations for an egress the policy does not name, got %v %v", dst4, dst6)
	}
}

// TestAddUserDeviceAclRule_DropsRuleWithoutDestination guards the netclient
// behavior where an omitted "-d" turns a rule into allow-all.
func TestAddUserDeviceAclRule_DropsRuleWithoutDestination(t *testing.T) {
	policy := models.Acl{ID: "alice-to-sig", Enabled: true}
	src4 := []net.IPNet{{IP: net.ParseIP("100.64.0.9"), Mask: net.CIDRMask(32, 32)}}

	rules := make(map[string]models.AclRule)
	addUserDeviceAclRule(rules, policy, src4, nil, nil, nil)
	if len(rules) != 0 {
		t.Fatalf("expected no rule when there is no destination, got %+v", rules)
	}

	addUserDeviceAclRule(rules, policy, nil, nil, src4, nil)
	if len(rules) != 0 {
		t.Fatalf("expected no rule when there is no source, got %+v", rules)
	}
}
