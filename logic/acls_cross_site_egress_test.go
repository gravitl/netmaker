package logic

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func TestCrossSiteEgressIPNetPairs_skipsReflexive(t *testing.T) {
	_, a, _ := net.ParseCIDR("10.110.0.0/20")
	_, b, _ := net.ParseCIDR("10.104.0.0/20")
	srcs := []net.IPNet{*a, *b}
	dsts := []net.IPNet{*a, *b}
	pairs := crossSiteEgressIPNetPairs(srcs, dsts)
	if len(pairs) != 2 {
		t.Fatalf("expected 2 cross pairs, got %d", len(pairs))
	}
}

func TestGetEgressAclRulesForTargetNode_VirtualNAT(t *testing.T) {
	srcID, dstID := uuid.New(), uuid.New()
	origEgressByID := getEgressByID
	origPolicies := getNetworkAccessDevicePolicies
	origNode := getNodeByIDForEgressFw
	t.Cleanup(func() {
		getEgressByID, getNetworkAccessDevicePolicies, getNodeByIDForEgressFw = origEgressByID, origPolicies, origNode
	})
	dstVirtual := "203.0.113.0/24"
	getEgressByID = func(id string) (schema.Egress, error) {
		switch id {
		case "src-egress":
			return schema.Egress{ID: id, Status: true, Nat: true, Mode: schema.VirtualNAT,
				Range: "192.0.2.0/25", VirtualRange: "192.0.2.128/25", Nodes: datatypes.JSONMap{srcID.String(): true}}, nil
		case "dst-egress":
			return schema.Egress{ID: id, Status: true, Nat: true, Mode: schema.VirtualNAT,
				Range: "198.51.100.0/24", VirtualRange: dstVirtual, Nodes: datatypes.JSONMap{dstID.String(): true}}, nil
		}
		return schema.Egress{}, errors.New("not found")
	}
	getNodeByIDForEgressFw = func(string) (models.Node, error) { return models.Node{}, errors.New("not found") }
	direction := models.TrafficDirectionUni
	getNetworkAccessDevicePolicies = func(context.Context, schema.NetworkID) []models.Acl {
		return []models.Acl{{
			ID: "s2s", Enabled: true, AllowedDirection: direction, Proto: models.ALL,
			Src: []models.AclPolicyTag{{ID: models.EgressID, Value: "src-egress"}},
			Dst: []models.AclPolicyTag{{ID: models.EgressID, Value: "dst-egress"}},
		}}
	}
	rulesFor := func(t *testing.T, target uuid.UUID) map[string]models.AclRule {
		t.Helper()
		return getEgressAclRulesForTargetNode(db.WithContext(context.Background()), models.Node{CommonNode: models.CommonNode{ID: target, Network: "n1"}})
	}
	for _, tt := range []struct {
		name, dstVirtual, wantSrc, wantDst string
		target                             uuid.UUID
	}{
		{"non-owner destination uses virtual range", "203.0.113.0/24", "192.0.2.0/25", "203.0.113.0/24", srcID},
		{"owner destination uses real range", "203.0.113.0/24", "192.0.2.128/25", "198.51.100.0/24", dstID},
		{"empty virtual range falls back to range", "", "192.0.2.0/25", "198.51.100.0/24", srcID},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dstVirtual = tt.dstVirtual
			rules := rulesFor(t, tt.target)
			rule, ok := rules["s2s#xs0"]
			if !ok {
				t.Fatalf("expected a forward rule, got %v", rules)
			}
			if len(rule.IPList) != 1 || rule.IPList[0].String() != tt.wantSrc {
				t.Errorf("expected source range %s, got %v", tt.wantSrc, rule.IPList)
			}
			if len(rule.Dst) != 1 || rule.Dst[0].String() != tt.wantDst {
				t.Errorf("expected destination range %s, got %v", tt.wantDst, rule.Dst)
			}
		})
	}
	t.Run("reverse leg uses virtual range as destination", func(t *testing.T) {
		dstVirtual = "203.0.113.0/24"
		direction = models.TrafficDirectionBi
		rule, ok := rulesFor(t, dstID)["s2s-reverse#xs0"]
		if !ok {
			t.Fatalf("expected a reverse rule, got %v", rulesFor(t, dstID))
		}
		if len(rule.Dst) != 1 || rule.Dst[0].String() != "192.0.2.128/25" {
			t.Errorf("expected reverse destination range 192.0.2.128/25, got %v", rule.Dst)
		}
	})
}

func TestGetEgressDefaultAllowAllFwRule(t *testing.T) {
	_, mesh, _ := net.ParseCIDR("100.64.0.0/16")
	_, lan, _ := net.ParseCIDR("10.104.0.0/20")
	n := models.Node{
		CommonNode: models.CommonNode{
			ID:      uuid.New(),
			Network: "n1",
			NetworkRange: net.IPNet{
				IP:   mesh.IP,
				Mask: mesh.Mask,
			},
		},
		EgressDetails: models.EgressDetails{
			IsEgressGateway: true,
			EgressGatewayRequest: models.EgressGatewayRequest{
				Ranges: []string{"10.104.0.0/20"},
			},
		},
	}
	r, ok := GetEgressDefaultAllowAllFwRule(n)
	if !ok {
		t.Fatal("expected rule")
	}
	if len(r.IPList) != 1 || r.IPList[0].String() != "100.64.0.0/16" {
		t.Fatalf("mesh src: got %v", r.IPList)
	}
	if len(r.Dst) != 1 || r.Dst[0].String() != lan.String() {
		t.Fatalf("lan dst: got %v", r.Dst)
	}
}

func TestGetEgressDefaultAllowAllFwRule_notEgress(t *testing.T) {
	n := models.Node{}
	_, ok := GetEgressDefaultAllowAllFwRule(n)
	if ok {
		t.Fatal("expected false")
	}
}
