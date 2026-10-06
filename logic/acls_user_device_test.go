package logic

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func TestNodesForAllResourcesTag_DropsUserDevices(t *testing.T) {
	infra := models.Node{CommonNode: models.CommonNode{ID: uuid.New()}}
	userDevice := models.Node{CommonNode: models.CommonNode{ID: uuid.New()}, OwnerID: "alice"}
	extClient := models.Node{
		IsStatic:   true,
		IsUserNode: true,
		StaticNode: models.ExtClient{OwnerID: "bob", RemoteAccessClientID: "mac"},
	}

	got := NodesForAllResourcesTag([]models.Node{infra, userDevice, extClient})
	if len(got) != 2 {
		t.Fatalf("expected user device to be dropped, got %d nodes", len(got))
	}
	for _, n := range got {
		if n.ID == userDevice.ID {
			t.Fatalf("user device must not be covered by all-resources tag, got %+v", got)
		}
	}
}

func TestNetworkHasUserDevices(t *testing.T) {
	infra := models.Node{CommonNode: models.CommonNode{ID: uuid.New()}}
	extClient := models.Node{
		IsStatic:   true,
		IsUserNode: true,
		StaticNode: models.ExtClient{OwnerID: "bob", RemoteAccessClientID: "mac"},
	}
	userDevice := models.Node{CommonNode: models.CommonNode{ID: uuid.New()}, OwnerID: "alice"}

	if NetworkHasUserDevices([]models.Node{infra, extClient}) {
		t.Fatal("infrastructure nodes and remote access clients must not count as user devices")
	}
	if !NetworkHasUserDevices([]models.Node{infra, extClient, userDevice}) {
		t.Fatal("expected host-backed user device to be detected")
	}
}

func TestCrossSiteEgressIPNetPairs_DropsDefaultRouteSources(t *testing.T) {
	_, inet, err := net.ParseCIDR("0.0.0.0/0")
	if err != nil {
		t.Fatal(err)
	}
	_, lan, err := net.ParseCIDR("10.104.0.0/20")
	if err != nil {
		t.Fatal(err)
	}
	if pairs := crossSiteEgressIPNetPairs([]net.IPNet{*inet}, []net.IPNet{*lan}); len(pairs) != 0 {
		t.Fatalf("0.0.0.0/0 as a site-to-site source would match user devices, got %+v", pairs)
	}
	pairs := crossSiteEgressIPNetPairs([]net.IPNet{*lan}, []net.IPNet{*inet})
	if len(pairs) != 1 || pairs[0].Src.String() != "10.104.0.0/20" || pairs[0].Dst.String() != "0.0.0.0/0" {
		t.Fatalf("LAN -> internet site-to-site must stay, got %+v", pairs)
	}
}

func TestIsNodeAllowedToCommunicateWithAllRsrcs_UserDeviceDenied(t *testing.T) {
	userDevice := models.Node{
		CommonNode: models.CommonNode{ID: uuid.New(), Network: "netmaker"},
		OwnerID:    "alice",
	}
	if IsNodeAllowedToCommunicateWithAllRsrcs(db.WithContext(context.Background()), userDevice) {
		t.Fatal("resource policies must never grant a user device access to all resources")
	}
}

// TestGetEgressRulesForNode_AllResourcesSrcExcludesUserDevice is the regression for
// the reported bug: a device policy whose source is All Resources used to put the
// whole mesh range in IPList, so a user device restricted to a single egress IP by
// its user policy still matched the rule granting the full egress range.
func TestGetEgressRulesForNode_AllResourcesSrcExcludesUserDevice(t *testing.T) {
	originalGetEgressByNetwork := getEgressByNetwork
	originalGetDevicePoliciesByNetwork := getDevicePoliciesByNetwork
	originalGetTagMap := GetTagMapWithNodesByNetwork
	t.Cleanup(func() {
		getEgressByNetwork = originalGetEgressByNetwork
		getDevicePoliciesByNetwork = originalGetDevicePoliciesByNetwork
		GetTagMapWithNodesByNetwork = originalGetTagMap
	})

	targetID := uuid.New()
	infraID := uuid.New()
	deviceID := uuid.New()
	targetNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      targetID,
			Network: "netmaker",
			Address: net.IPNet{IP: net.ParseIP("100.64.0.5"), Mask: net.CIDRMask(32, 32)},
			NetworkRange: net.IPNet{
				IP:   net.ParseIP("100.64.0.0"),
				Mask: net.CIDRMask(16, 32),
			},
		},
		Tags: map[models.TagID]struct{}{},
	}
	infraNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      infraID,
			Network: "netmaker",
			Address: net.IPNet{IP: net.ParseIP("100.64.0.7"), Mask: net.CIDRMask(32, 32)},
		},
	}
	userDevice := models.Node{
		CommonNode: models.CommonNode{
			ID:      deviceID,
			Network: "netmaker",
			Address: net.IPNet{IP: net.ParseIP("100.64.0.9"), Mask: net.CIDRMask(32, 32)},
		},
		OwnerID: "alice",
	}

	getEgressByNetwork = func(network string) ([]schema.Egress, error) {
		return []schema.Egress{{
			ID:      "sig",
			Network: network,
			Status:  true,
			Range:   "10.104.0.0/20",
			Nodes:   datatypes.JSONMap{targetID.String(): json.Number("100")},
		}}, nil
	}
	getDevicePoliciesByNetwork = func(ctx context.Context, netID schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID:               "all-rsrcs-to-sig",
			Enabled:          true,
			AllowedDirection: models.TrafficDirectionUni,
			Proto:            models.ALL,
			Src:              []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}},
			Dst:              []models.AclPolicyTag{{ID: models.EgressID, Value: "sig"}},
		}}
	}
	GetTagMapWithNodesByNetwork = func(ctx context.Context, netID schema.NetworkID, withStatic bool) map[models.TagID][]models.Node {
		return map[models.TagID][]models.Node{
			"*": NodesForAllResourcesTag([]models.Node{targetNode, infraNode, userDevice}),
		}
	}

	rules := GetEgressRulesForNode(db.WithContext(context.Background()), targetNode)
	rule, ok := rules["all-rsrcs-to-sig"]
	if !ok {
		t.Fatalf("expected rule keyed by acl.ID, got: %+v", rules)
	}
	srcs := make(map[string]struct{}, len(rule.IPList))
	for _, n := range rule.IPList {
		srcs[n.String()] = struct{}{}
	}
	if _, ok := srcs["100.64.0.7/32"]; !ok {
		t.Fatalf("expected infra node /32 as source, got %v", rule.IPList)
	}
	if _, ok := srcs["100.64.0.9/32"]; ok {
		t.Fatalf("user device must not be a source of an all-resources device policy, got %v", rule.IPList)
	}
	if _, ok := srcs["100.64.0.0/16"]; ok {
		t.Fatalf("expected enumerated sources instead of the mesh range, got %v", rule.IPList)
	}
	dsts := make(map[string]struct{}, len(rule.Dst))
	for _, n := range rule.Dst {
		dsts[n.String()] = struct{}{}
	}
	if _, ok := dsts["10.104.0.0/20"]; !ok {
		t.Fatalf("expected infra sources to keep the full egress range, got %v", rule.Dst)
	}
}

// TestGetAclRuleForInetGw_ExcludesUserDevices covers the internet-egress twin: this
// rule's destination is every address, so including a user device in its source
// would override whatever its user policy restricts it to.
func TestGetAclRuleForInetGw_ExcludesUserDevices(t *testing.T) {
	originalGetTagMap := GetTagMapWithNodesByNetwork
	t.Cleanup(func() { GetTagMapWithNodesByNetwork = originalGetTagMap })

	gwID := uuid.New()
	gateway := models.Node{
		CommonNode: models.CommonNode{
			ID:      gwID,
			Network: "netmaker",
			NetworkRange: net.IPNet{
				IP:   net.ParseIP("100.64.0.0"),
				Mask: net.CIDRMask(16, 32),
			},
		},
		IsInternetGateway: true,
	}
	infraNode := models.Node{
		CommonNode: models.CommonNode{
			ID:      uuid.New(),
			Network: "netmaker",
			Address: net.IPNet{IP: net.ParseIP("100.64.0.7"), Mask: net.CIDRMask(32, 32)},
		},
	}
	userDevice := models.Node{
		CommonNode: models.CommonNode{
			ID:      uuid.New(),
			Network: "netmaker",
			Address: net.IPNet{IP: net.ParseIP("100.64.0.9"), Mask: net.CIDRMask(32, 32)},
		},
		OwnerID: "alice",
	}
	GetTagMapWithNodesByNetwork = func(ctx context.Context, netID schema.NetworkID, withStatic bool) map[models.TagID][]models.Node {
		return map[models.TagID][]models.Node{
			"*": NodesForAllResourcesTag([]models.Node{gateway, infraNode, userDevice}),
		}
	}

	rules := GetAclRuleForInetGw(gateway)
	rule, ok := rules[gwID.String()+"-inet-gw-internal-rule"]
	if !ok {
		t.Fatalf("expected inet gw rule, got: %+v", rules)
	}
	srcs := make(map[string]struct{}, len(rule.IPList))
	for _, n := range rule.IPList {
		srcs[n.String()] = struct{}{}
	}
	if _, ok := srcs["100.64.0.7/32"]; !ok {
		t.Fatalf("expected infra node /32 as source, got %v", rule.IPList)
	}
	if _, ok := srcs["100.64.0.9/32"]; ok {
		t.Fatalf("user device must not get 0.0.0.0/0 from the inet gw rule, got %v", rule.IPList)
	}
	if _, ok := srcs["100.64.0.0/16"]; ok {
		t.Fatalf("expected enumerated sources instead of the mesh range, got %v", rule.IPList)
	}
}

func TestAppendNodeIPsToAclRule_SkipsSelfAndUserDevices(t *testing.T) {
	selfID := uuid.New()
	nodes := []models.Node{
		{CommonNode: models.CommonNode{
			ID:      selfID,
			Address: net.IPNet{IP: net.ParseIP("100.64.0.5"), Mask: net.CIDRMask(32, 32)},
		}},
		{CommonNode: models.CommonNode{
			ID:       uuid.New(),
			Address:  net.IPNet{IP: net.ParseIP("100.64.0.7"), Mask: net.CIDRMask(32, 32)},
			Address6: net.IPNet{IP: net.ParseIP("fd00::7"), Mask: net.CIDRMask(128, 128)},
		}},
		{
			CommonNode: models.CommonNode{
				ID:      uuid.New(),
				Address: net.IPNet{IP: net.ParseIP("100.64.0.9"), Mask: net.CIDRMask(32, 32)},
			},
			OwnerID: "alice",
		},
		{
			IsStatic:   true,
			IsUserNode: true,
			StaticNode: models.ExtClient{OwnerID: "bob", RemoteAccessClientID: "mac", Address: "100.64.0.11"},
		},
	}

	var rule models.AclRule
	appendNodeIPsToAclRule(&rule, nodes, selfID.String())

	got := make(map[string]struct{}, len(rule.IPList))
	for _, n := range rule.IPList {
		got[n.String()] = struct{}{}
	}
	if _, ok := got["100.64.0.5/32"]; ok {
		t.Fatalf("expected the rule's own node to be skipped, got %v", rule.IPList)
	}
	if _, ok := got["100.64.0.9/32"]; ok {
		t.Fatalf("expected user device to be skipped, got %v", rule.IPList)
	}
	for _, want := range []string{"100.64.0.7/32", "100.64.0.11/32"} {
		if _, ok := got[want]; !ok {
			t.Fatalf("expected %s in sources, got %v", want, rule.IPList)
		}
	}
	if len(rule.IP6List) != 1 || rule.IP6List[0].String() != "fd00::7/128" {
		t.Fatalf("expected the infra node's v6 address, got %v", rule.IP6List)
	}
}
