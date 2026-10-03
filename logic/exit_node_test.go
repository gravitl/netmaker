package logic

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func TestExitNodeAllowedEndpoints(t *testing.T) {
	host := &schema.Host{
		EndpointIP:   net.ParseIP("203.0.113.10"),
		EndpointIPv6: net.ParseIP("2001:db8::1"),
	}
	got := exitNodeAllowedEndpoints(host)
	if len(got) != 2 {
		t.Fatalf("expected 2 public endpoints, got %v", got)
	}
	if got[0] != "203.0.113.10" || got[1] != "2001:db8::1" {
		t.Fatalf("unexpected endpoints %v", got)
	}
	if n := exitNodeAllowedEndpoints(nil); len(n) != 0 {
		t.Fatalf("expected no endpoints, got %v", n)
	}
}

func TestListNodeExitNodes_Validation(t *testing.T) {
	_, err := ListNodeExitNodes(context.Background(), "", "node-1")
	if err == nil || err.Error() != "network and node are required" {
		t.Fatalf("expected validation error, got %v", err)
	}
	_, err = ListNodeExitNodes(context.Background(), "net-1", "")
	if err == nil || err.Error() != "network and node are required" {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestAssignNodeExitNode_Validation(t *testing.T) {
	_, err := AssignNodeExitNode(context.Background(), "", "node-1", "eg-1", false)
	if err == nil || err.Error() != "network and node are required" {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestPickFallbackExitNode(t *testing.T) {
	exits := []models.DeviceExitNode{
		{EgressID: "b", Name: "bravo", Status: true},
		{EgressID: "a", Name: "alpha", Status: true},
		{EgressID: "off", Name: "aaa", Status: false},
	}
	if _, ok := pickFallbackExitNode("a", exits); ok {
		t.Fatal("valid selection must be kept")
	}
	pick, ok := pickFallbackExitNode("", exits)
	if !ok || pick.EgressID != "a" {
		t.Fatalf("expected alpha, got %+v ok=%v", pick, ok)
	}
	pick, ok = pickFallbackExitNode("missing", exits)
	if !ok || pick.EgressID != "a" {
		t.Fatalf("expected fallback alpha, got %+v ok=%v", pick, ok)
	}
	if _, ok := pickFallbackExitNode("", nil); ok {
		t.Fatal("no exits must not assign")
	}
}

func TestErrExitNodeSelectionRequired(t *testing.T) {
	if ErrExitNodeSelectionRequired.Error() != "exit node selection is required" {
		t.Fatalf("unexpected error %v", ErrExitNodeSelectionRequired)
	}
}

func TestValidateInternetEgressSelection_ExitNodeCannotUseAnotherExitNode(t *testing.T) {
	node := &models.Node{}
	node.ID = uuid.New()
	node.Network = "net-1"

	err := validateInternetEgressSelection(node, &schema.Node{ID: node.ID.String()}, uuid.NewString(), true)
	if err == nil || err.Error() != "exit node cannot use another exit node" {
		t.Fatalf("expected exit-node validation error, got %v", err)
	}
}
