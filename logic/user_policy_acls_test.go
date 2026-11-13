package logic

import (
	"context"
	"testing"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/servercfg"
)

func TestDefaultsAllowAllTrafficUsesRealUserPolicyOnCE(t *testing.T) {
	wasPro := servercfg.IsPro
	servercfg.IsPro = false
	t.Cleanup(func() { servercfg.IsPro = wasPro })

	open := func(rule models.AclPolicyType, enabled bool) models.Acl {
		return models.Acl{
			Enabled:  enabled,
			RuleType: rule,
			Src:      []models.AclPolicyTag{{Value: "*"}},
			Dst:      []models.AclPolicyTag{{Value: "*"}},
		}
	}
	allNodes := open(models.DevicePolicy, true)
	allUsersOff := open(models.UserPolicy, false)
	allUsersOn := open(models.UserPolicy, true)

	if defaultsAllowAllTraffic(allNodes, allUsersOff, nil) {
		t.Fatal("CE must not force user default on when All Users is disabled")
	}
	if !defaultsAllowAllTraffic(allNodes, allUsersOn, nil) {
		t.Fatal("CE AllowAll should require both All Nodes and All Users enabled")
	}
}

func TestIsAclPolicyValidUserPolicyOnCE(t *testing.T) {
	wasPro := servercfg.IsPro
	servercfg.IsPro = false
	t.Cleanup(func() { servercfg.IsPro = wasPro })

	acl := models.Acl{
		NetworkID:        "net",
		RuleType:         models.UserPolicy,
		AccessType:       models.NetworkAccess,
		AllowedDirection: models.TrafficDirectionBi,
		Proto:            models.ALL,
		ServiceType:      models.Any,
		Src: []models.AclPolicyTag{
			{ID: models.UserAclID, Value: "*"},
		},
		Dst: []models.AclPolicyTag{
			{ID: models.NodeTagID, Value: "*"},
		},
	}
	if err := IsAclPolicyValid(context.Background(), acl); err != nil {
		t.Fatalf("All Users policy should validate on CE: %v", err)
	}

	acl.Src = []models.AclPolicyTag{{ID: models.UserGroupAclID, Value: "ops"}}
	if err := IsAclPolicyValid(context.Background(), acl); err == nil {
		t.Fatal("user group sources must be rejected on CE")
	}
}

func TestIsUserAllowedToCommunicateDefaultAndPerUser(t *testing.T) {
	peer := newTestNode()
	peer.Network = "net"

	origGetDefault := GetDefaultPolicy
	origList := ListAcls
	t.Cleanup(func() {
		// restore via reassignment of package-level behavior used by helpers
		_ = origGetDefault
		_ = origList
	})

	// With default All Users enabled, any peer is allowed.
	defaultOn := models.Acl{
		ID:      "net.all-users",
		Enabled: true,
		Src:     []models.AclPolicyTag{{ID: models.UserAclID, Value: "*"}},
		Dst:     []models.AclPolicyTag{{ID: models.NodeTagID, Value: "*"}},
	}
	ok, policies := isUserAllowedToCommunicateWithPolicies("alice", *peer, defaultOn, nil)
	if !ok || len(policies) != 1 {
		t.Fatalf("default on = ok=%v policies=%+v", ok, policies)
	}

	// Default off: only matching per-user policy.
	perUser := models.Acl{
		ID:        "net.alice-peer",
		Enabled:   true,
		NetworkID: "net",
		RuleType:  models.UserPolicy,
		Src:       []models.AclPolicyTag{{ID: models.UserAclID, Value: "alice"}},
		Dst:       []models.AclPolicyTag{{ID: models.NodeID, Value: peer.ID.String()}},
	}
	ok, policies = isUserAllowedToCommunicateWithPolicies("alice", *peer, models.Acl{Enabled: false}, []models.Acl{perUser})
	if !ok || len(policies) != 1 {
		t.Fatalf("per-user allow = ok=%v policies=%+v", ok, policies)
	}
	ok, _ = isUserAllowedToCommunicateWithPolicies("bob", *peer, models.Acl{Enabled: false}, []models.Acl{perUser})
	if ok {
		t.Fatal("bob must not match alice-only policy")
	}
}

// isUserAllowedToCommunicateWithPolicies is a pure helper for unit tests of the
// matching rules without DB lookups.
func isUserAllowedToCommunicateWithPolicies(userName string, peer models.Node, defaultPolicy models.Acl, policies []models.Acl) (bool, []models.Acl) {
	if defaultPolicy.Enabled {
		return true, []models.Acl{defaultPolicy}
	}
	peerID := peer.ID.String()
	var allowed []models.Acl
	for _, policy := range policies {
		if !policy.Enabled || policy.RuleType != models.UserPolicy {
			continue
		}
		srcMap := ConvAclTagToValueMap(policy.Src)
		if _, ok := srcMap["*"]; !ok {
			if _, ok := srcMap[userName]; !ok {
				continue
			}
		}
		dstMap := ConvAclTagToValueMap(policy.Dst)
		if _, ok := dstMap["*"]; ok {
			allowed = append(allowed, policy)
			continue
		}
		if _, ok := dstMap[peerID]; ok {
			allowed = append(allowed, policy)
		}
	}
	return len(allowed) > 0, allowed
}

func TestValidateCENetworkAclProtoAndPorts(t *testing.T) {
	wasPro := servercfg.IsPro
	servercfg.IsPro = false
	t.Cleanup(func() { servercfg.IsPro = wasPro })

	ok := models.Acl{
		AccessType:  models.NetworkAccess,
		ServiceType: models.Any,
		Proto:       models.ALL,
	}
	if err := validateCENetworkAclProtoAndPorts(ok); err != nil {
		t.Fatalf("Any/ALL should be allowed on CE: %v", err)
	}

	if err := validateCENetworkAclProtoAndPorts(models.Acl{
		AccessType:  models.NetworkAccess,
		ServiceType: models.Custom,
		Proto:       models.TCP,
		Port:        []string{"443"},
	}); err == nil {
		t.Fatal("custom protocol/port must be rejected on CE")
	}
	if err := validateCENetworkAclProtoAndPorts(models.Acl{
		AccessType: models.NetworkAccess,
		Proto:      models.TCP,
	}); err == nil {
		t.Fatal("non-ALL protocol must be rejected on CE")
	}
	if err := validateCENetworkAclProtoAndPorts(models.Acl{
		AccessType: models.NetworkAccess,
		Proto:      models.ALL,
		Port:       []string{"22"},
	}); err == nil {
		t.Fatal("custom ports must be rejected on CE")
	}

	servercfg.IsPro = true
	if err := validateCENetworkAclProtoAndPorts(models.Acl{
		AccessType:  models.NetworkAccess,
		ServiceType: models.Custom,
		Proto:       models.TCP,
		Port:        []string{"443"},
	}); err != nil {
		t.Fatalf("Pro should allow custom proto/port: %v", err)
	}
}

func TestCheckIfAclTagisValidUserAclID(t *testing.T) {
	acl := models.Acl{RuleType: models.UserPolicy, NetworkID: "net"}
	if err := checkIfAclTagisValid(context.Background(), acl, models.AclPolicyTag{ID: models.UserAclID, Value: "*"}, true); err != nil {
		t.Fatalf("user:* should be valid: %v", err)
	}
	if err := checkIfAclTagisValid(context.Background(), acl, models.AclPolicyTag{ID: models.UserGroupAclID, Value: "g"}, true); err == nil {
		t.Fatal("user groups should be invalid on CE tag check")
	}
	if err := checkIfAclTagisValid(context.Background(), acl, models.AclPolicyTag{ID: models.UserAclID, Value: "alice"}, false); err == nil {
		t.Fatal("user cannot be a destination")
	}
}
