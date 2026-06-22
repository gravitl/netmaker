package logic

import (
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
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
