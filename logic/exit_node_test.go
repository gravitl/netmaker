package logic

import (
	"context"
	"encoding/json"
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

func TestDeviceMaySelectExitNode_UserDeviceIgnoresAllResources(t *testing.T) {
	user := &schema.User{Username: "abhi"}
	host := &schema.Host{OwnerUsername: "abhi"}
	node := &models.Node{
		CommonNode: models.CommonNode{ID: uuid.New(), Network: "netmaker"},
		OwnerID:    "abhi",
	}
	exit := &schema.Egress{ID: "sig-exit", Status: true, Type: schema.EgressTypeInternet, Range: "*"}
	allowed := []models.Acl{{
		Enabled: true,
		Src:     []models.AclPolicyTag{{ID: models.UserAclID, Value: "abhi"}},
		Dst:     []models.AclPolicyTag{{ID: models.EgressID, Value: "sig-exit"}},
	}}
	otherExit := &schema.Egress{ID: "other-exit", Status: true, Type: schema.EgressTypeInternet, Range: "*"}

	// All Resources on, All Users off: only exits granted by user policy.
	if !deviceMaySelectExitNode(user, host, node, exit, nil, allowed, true, false) {
		t.Fatal("user policy must still allow the granted exit when All Resources is on")
	}
	if deviceMaySelectExitNode(user, host, node, otherExit, nil, allowed, true, false) {
		t.Fatal("All Resources must not expose exits the user policy does not grant")
	}
	// All Users on: every exit is selectable.
	if !deviceMaySelectExitNode(user, host, node, otherExit, nil, allowed, true, true) {
		t.Fatal("All Users should allow every exit for user devices")
	}
	// Infra host still gets All Resources free pass.
	infraHost := &schema.Host{Name: "server"}
	infraNode := &models.Node{CommonNode: models.CommonNode{ID: uuid.New(), Network: "netmaker"}}
	if !deviceMaySelectExitNode(user, infraHost, infraNode, otherExit, nil, nil, true, false) {
		t.Fatal("infra devices may use All Resources for exit listing")
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
	// Current selection Status=false (disconnected routing) → reassign.
	pick, ok = pickFallbackExitNode("off", exits)
	if !ok || pick.EgressID != "a" {
		t.Fatalf("expected reassign from down exit to alpha, got %+v ok=%v", pick, ok)
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

func TestDeviceExitNodeSelectionReqForceJSON(t *testing.T) {
	// Ensure the force flag is part of the public device API contract.
	raw := []byte(`{"egress_id":"","force":true}`)
	var req models.DeviceExitNodeSelectionReq
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatal(err)
	}
	if !req.Force || req.EgressID != "" {
		t.Fatalf("unexpected req %+v", req)
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
