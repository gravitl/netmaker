package logic

import (
	"context"
	"maps"
	"net"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

const srcEgressNet = "src-egress-net"

type srcEgressFixture struct {
	peer, peer6                                                      models.Node
	siteToPeer, siteToRouter, mixed, inetToPeer, wild, defaultToPeer models.Acl
	dualToPeer, dualToPeer6                                          models.Acl
}

// newSrcEgressFixture builds network src-egress-net, its egresses, the peers
// the policies target and stubs the rule builder's lookups; the default policy
// is removed on cleanup.
func newSrcEgressFixture(t *testing.T) srcEgressFixture {
	t.Helper()
	origByID, origByNet, origPol := getEgressByID, getEgressByNetwork, getNetworkAccessDevicePolicies
	origTags, origTagNodes := GetTagMapWithNodesByNetwork, GetTagMapWithNetworkNodes
	t.Cleanup(func() {
		getEgressByID, getEgressByNetwork, getNetworkAccessDevicePolicies = origByID, origByNet, origPol
		GetTagMapWithNodesByNetwork, GetTagMapWithNetworkNodes = origTags, origTagNodes
		_ = DeleteAcl(db.WithContext(context.Background()), models.Acl{ID: srcEgressNet + ".all-nodes", NetworkID: srcEgressNet})
	})
	router := models.Node{CommonNode: models.CommonNode{ID: uuid.New(), Network: srcEgressNet, Address: net.IPNet{IP: net.ParseIP("100.64.0.4"), Mask: net.CIDRMask(32, 32)}}}
	peer := models.Node{CommonNode: models.CommonNode{ID: uuid.New(), Network: srcEgressNet, Address: net.IPNet{IP: net.ParseIP("100.64.0.5"), Mask: net.CIDRMask(32, 32)}}}
	peer6 := models.Node{CommonNode: models.CommonNode{ID: uuid.New(), Network: srcEgressNet, Address6: net.IPNet{IP: net.ParseIP("2001:db8::5"), Mask: net.CIDRMask(128, 128)}}}
	owner := datatypes.JSONMap{router.ID.String(): 1}
	site := schema.Egress{ID: "sites", Network: srcEgressNet, Status: true, Range: "203.0.113.0/24", Nodes: owner}
	site6 := schema.Egress{ID: "v6-sites", Network: srcEgressNet, Status: true, Range: "2001:db8:1::/64", Nodes: owner}
	natSite := schema.Egress{ID: "nat-sites", Network: srcEgressNet, Status: true, Nat: true, Range: "198.51.100.0/24", Nodes: owner}
	internet := schema.Egress{ID: "internet", Network: srcEgressNet, Status: true, Range: "*", Nodes: owner}
	defaultRoute := schema.Egress{ID: "default-route", Network: srcEgressNet, Status: true, Type: schema.EgressTypeCIDR, Range: "0.0.0.0/0", Nodes: owner}
	egresses := map[string]schema.Egress{site.ID: site, site6.ID: site6, natSite.ID: natSite, internet.ID: internet, defaultRoute.ID: defaultRoute}
	egressList := slices.AppendSeq(make([]schema.Egress, 0, len(egresses)), maps.Values(egresses))
	getEgressByID = func(id string) (schema.Egress, error) { return egresses[id], nil }
	getEgressByNetwork = func(string) ([]schema.Egress, error) { return egressList, nil }
	tags := map[models.TagID][]models.Node{"*": {router, peer, peer6}, models.TagID(router.ID.String()): {router}, models.TagID(peer.ID.String()): {peer}, models.TagID(peer6.ID.String()): {peer6}}
	GetTagMapWithNodesByNetwork = func(context.Context, schema.NetworkID, bool) map[models.TagID][]models.Node { return tags }
	GetTagMapWithNetworkNodes = func(context.Context, schema.NetworkID, []models.Node, bool) map[models.TagID][]models.Node {
		return tags
	}
	peerDst := []models.AclPolicyTag{{ID: models.NodeID, Value: peer.ID.String()}}
	siteSrc := []models.AclPolicyTag{{ID: models.EgressID, Value: site.ID}, {ID: models.NetmakerIPAclID, Value: "203.0.113.10/32"}}
	policy := func(id string, src, dst []models.AclPolicyTag) models.Acl {
		return models.Acl{ID: id, NetworkID: srcEgressNet, RuleType: models.DevicePolicy, Enabled: true,
			AllowedDirection: models.TrafficDirectionUni, Proto: models.TCP, Port: []string{"22"}, Src: src, Dst: dst}
	}
	dualSrc := []models.AclPolicyTag{{ID: models.EgressID, Value: site.ID}, {ID: models.EgressID, Value: site6.ID}}
	return srcEgressFixture{peer: peer, peer6: peer6,
		siteToPeer:    policy("site-to-peer", siteSrc, peerDst),
		siteToRouter:  policy("site-to-router", siteSrc, []models.AclPolicyTag{{ID: models.NodeID, Value: router.ID.String()}}),
		mixed:         policy("mixed-to-peer", []models.AclPolicyTag{{ID: models.EgressID, Value: site.ID}, {ID: models.EgressID, Value: natSite.ID}}, peerDst),
		inetToPeer:    policy("inet-to-peer", []models.AclPolicyTag{{ID: models.EgressID, Value: internet.ID}}, peerDst),
		wild:          policy("wild", []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}}, peerDst),
		defaultToPeer: policy("default-to-peer", []models.AclPolicyTag{{ID: models.EgressID, Value: defaultRoute.ID}}, peerDst),
		dualToPeer:    policy("dual-to-peer", dualSrc, peerDst),
		dualToPeer6:   policy("dual-to-peer6", dualSrc, []models.AclPolicyTag{{ID: models.NodeID, Value: peer6.ID.String()}}),
	}
}

// TestSrcEgressNonNATPolicyRules checks that a policy naming the node gets a
// #src-egress-in rule for the non-NAT source egresses it names, scoped to the
// policy protocol and ports and to the node's own addresses of the families it
// has, and none for wildcard, internet and default route sources; a NAT egress
// mixed into the source adds no range of its own.
func TestSrcEgressNonNATPolicyRules(t *testing.T) {
	f := newSrcEgressFixture(t)
	cases := []struct {
		node                *models.Node
		policies            []models.Acl
		defaultOn, wildcard bool
		want                string
		no                  []string
		wantNet, noNet      string
	}{
		{&f.peer, []models.Acl{f.siteToPeer}, true, false, f.siteToPeer.ID + srcEgressInputSuffix, nil, "203.0.113.10/32", ""},
		{&f.peer, []models.Acl{f.siteToRouter, f.wild, f.defaultToPeer, f.inetToPeer}, true, false, "", []string{f.siteToRouter.ID + srcEgressInputSuffix, f.wild.ID + srcEgressInputSuffix, f.defaultToPeer.ID + srcEgressInputSuffix, f.inetToPeer.ID + srcEgressInputSuffix}, "", ""},
		{&f.peer, []models.Acl{f.wild}, true, true, "", []string{f.wild.ID + srcEgressInputSuffix}, "", ""},
		{&f.peer, []models.Acl{f.mixed}, true, false, f.mixed.ID + srcEgressInputSuffix, nil, "203.0.113.0/24", "198.51.100.0/24"},
		{&f.peer, []models.Acl{f.siteToPeer}, false, false, f.siteToPeer.ID, []string{f.siteToPeer.ID + srcEgressInputSuffix}, "", ""},
		{&f.peer, []models.Acl{f.dualToPeer}, true, false, f.dualToPeer.ID + srcEgressInputSuffix, nil, "203.0.113.0/24", "2001:db8:1::/64"},
		{&f.peer6, []models.Acl{f.dualToPeer6}, true, false, f.dualToPeer6.ID + srcEgressInputSuffix, nil, "2001:db8:1::/64", "203.0.113.0/24"},
	}
	ctx := db.WithContext(context.Background())
	for _, tc := range cases {
		defaultAcl := models.Acl{ID: srcEgressNet + ".all-nodes", NetworkID: srcEgressNet, RuleType: models.DevicePolicy, Enabled: tc.defaultOn}
		if tc.wildcard {
			defaultAcl.Src, defaultAcl.Dst = []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}}, []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}}
		}
		if err := UpsertAcl(ctx, defaultAcl); err != nil {
			t.Fatal(err)
		}
		getNetworkAccessDevicePolicies = func(context.Context, schema.NetworkID) []models.Acl { return tc.policies }
		rules := GetAclRulesForNode(ctx, NewPeerGraph(nil, nil), tc.node, nil)
		has := func(list []net.IPNet, s string) bool {
			return slices.ContainsFunc(list, func(n net.IPNet) bool { return n.String() == s })
		}
		allowAll := rules[tc.node.ID.String()+"-all-allowed-node-rule"]
		if (allowAll.ID != "") != tc.defaultOn || has(allowAll.IPList, "203.0.113.10/32") || has(allowAll.IPList, "0.0.0.0/0") {
			t.Fatalf("all-protocol rule present = %v, IPList %v", allowAll.ID != "", allowAll.IPList)
		}
		for _, id := range tc.no {
			if _, ok := rules[id]; ok {
				t.Fatalf("unexpected rule %s", id)
			}
		}
		if tc.want != "" {
			rule, ok := rules[tc.want]
			if !ok {
				t.Fatalf("expected rule %q at the node, got %+v", tc.want, rule)
			}
			dst4, dst6 := (&net.IPNet{IP: tc.node.Address.IP, Mask: net.CIDRMask(32, 32)}).String(), (&net.IPNet{IP: tc.node.Address6.IP, Mask: net.CIDRMask(128, 128)}).String()
			if (tc.wantNet != "" && !has(rule.IPList, tc.wantNet) && !has(rule.IP6List, tc.wantNet)) || (tc.noNet != "" && (has(rule.IPList, tc.noNet) || has(rule.IP6List, tc.noNet))) || tc.defaultOn && (rule.AllowedProtocol != models.TCP || len(rule.AllowedPorts) != 1 || rule.AllowedPorts[0] != "22" || (tc.node.Address.IP != nil) != has(rule.Dst, dst4) || (tc.node.Address6.IP != nil) != has(rule.Dst6, dst6) || len(rule.Dst)+len(rule.Dst6) != 1 || (tc.node.Address.IP != nil) != (len(rule.IPList) == 1)) {
				t.Fatalf("rule %q not scoped to tcp/22 at the node's own addresses: %+v", tc.want, rule)
			}
		}
	}
}
