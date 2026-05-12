package logic

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

func TestValidateSiteToSiteEgressNatForAcl_Nil(t *testing.T) {
	if err := ValidateSiteToSiteEgressNatForAcl(nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidateSiteToSiteEgressNatForAcl_UserPolicyIgnored(t *testing.T) {
	acl := models.Acl{
		RuleType: models.UserPolicy,
		Src:      []models.AclPolicyTag{{ID: models.EgressID, Value: "a"}, {ID: models.EgressID, Value: "b"}},
		Dst:      []models.AclPolicyTag{{ID: models.EgressID, Value: "c"}},
	}
	if err := ValidateSiteToSiteEgressNatForAcl(&acl); err != nil {
		t.Fatalf("expected nil for user policy, got %v", err)
	}
}

func TestValidateSiteToSiteEgressNatForAcl_SameEgressBothSides(t *testing.T) {
	originalGetEgressByID := getEgressByID
	t.Cleanup(func() { getEgressByID = originalGetEgressByID })
	getEgressByID = func(egressID string) (schema.Egress, error) {
		return schema.Egress{ID: egressID, Nat: true, Name: "only-one"}, nil
	}

	acl := models.Acl{
		ID:       uuid.New().String(),
		Name:     "self",
		RuleType: models.DevicePolicy,
		Src:      []models.AclPolicyTag{{ID: models.EgressID, Value: "eg-same"}},
		Dst:      []models.AclPolicyTag{{ID: models.EgressID, Value: "eg-same"}},
	}
	if err := ValidateSiteToSiteEgressNatForAcl(&acl); err != nil {
		t.Fatalf("same egress on src and dst should not require NAT check, got %v", err)
	}
}

func TestValidateSiteToSiteEgressNatForAcl_WildcardNoExplicitPair(t *testing.T) {
	originalGetEgressByID := getEgressByID
	t.Cleanup(func() { getEgressByID = originalGetEgressByID })
	getEgressByID = func(egressID string) (schema.Egress, error) {
		return schema.Egress{}, errors.New("unexpected")
	}

	acl := models.Acl{
		ID:       uuid.New().String(),
		Name:     "star",
		RuleType: models.DevicePolicy,
		Src:      []models.AclPolicyTag{{ID: models.EgressID, Value: "*"}},
		Dst:      []models.AclPolicyTag{{ID: models.EgressID, Value: "eg-b"}},
	}
	if err := ValidateSiteToSiteEgressNatForAcl(&acl); err != nil {
		t.Fatalf("wildcard src has no explicit egress id; expected nil, got %v", err)
	}
}

func TestValidateSiteToSiteEgressNatForAcl_CrossEgressBothNatOff(t *testing.T) {
	originalGetEgressByID := getEgressByID
	t.Cleanup(func() { getEgressByID = originalGetEgressByID })
	getEgressByID = func(egressID string) (schema.Egress, error) {
		return schema.Egress{ID: egressID, Nat: false, Name: egressID}, nil
	}

	acl := models.Acl{
		ID:       uuid.New().String(),
		Name:     "ok",
		RuleType: models.DevicePolicy,
		Src:      []models.AclPolicyTag{{ID: models.EgressID, Value: "eg-a"}},
		Dst:      []models.AclPolicyTag{{ID: models.EgressRange, Value: "eg-b"}},
	}
	if err := ValidateSiteToSiteEgressNatForAcl(&acl); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidateSiteToSiteEgressNatForAcl_CrossEgressRejectsNatOn(t *testing.T) {
	originalGetEgressByID := getEgressByID
	t.Cleanup(func() { getEgressByID = originalGetEgressByID })
	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "eg-a":
			return schema.Egress{ID: egressID, Nat: false, Name: "A"}, nil
		case "eg-b":
			return schema.Egress{ID: egressID, Nat: true, Name: "B-side"}, nil
		default:
			return schema.Egress{}, errors.New("not found")
		}
	}

	acl := models.Acl{
		ID:       uuid.New().String(),
		Name:     "s2s",
		RuleType: models.DevicePolicy,
		Src:      []models.AclPolicyTag{{ID: models.EgressID, Value: "eg-a"}},
		Dst:      []models.AclPolicyTag{{ID: models.EgressID, Value: "eg-b"}},
	}
	if err := ValidateSiteToSiteEgressNatForAcl(&acl); err == nil {
		t.Fatal("expected error when egress has NAT enabled")
	}
}

func TestValidateEgressNatAllowedForSiteToSitePolicies(t *testing.T) {
	origList := listDevicePoliciesForSiteToSiteNatCheck
	t.Cleanup(func() { listDevicePoliciesForSiteToSiteNatCheck = origList })

	if err := ValidateEgressNatAllowedForSiteToSitePolicies("eg-a", "net1", false); err != nil {
		t.Fatalf("enablingNat false: expected nil, got %v", err)
	}

	listDevicePoliciesForSiteToSiteNatCheck = func(netID schema.NetworkID) []models.Acl {
		if string(netID) != "net1" {
			return nil
		}
		return []models.Acl{
			{
				ID:       "pol-1",
				Name:     "cross",
				Enabled:  true,
				RuleType: models.DevicePolicy,
				Src:      []models.AclPolicyTag{{ID: models.EgressID, Value: "eg-a"}},
				Dst:      []models.AclPolicyTag{{ID: models.EgressID, Value: "eg-b"}},
			},
		}
	}
	if err := ValidateEgressNatAllowedForSiteToSitePolicies("eg-a", "net1", true); err == nil {
		t.Fatal("expected error when enabling NAT and policy references this egress")
	}

	listDevicePoliciesForSiteToSiteNatCheck = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{
			{
				ID:       "pol-2",
				Name:     "disabled",
				Enabled:  false,
				RuleType: models.DevicePolicy,
				Src:      []models.AclPolicyTag{{ID: models.EgressID, Value: "eg-a"}},
				Dst:      []models.AclPolicyTag{{ID: models.EgressID, Value: "eg-b"}},
			},
		}
	}
	if err := ValidateEgressNatAllowedForSiteToSitePolicies("eg-a", "net1", true); err != nil {
		t.Fatalf("disabled policy: expected nil, got %v", err)
	}

	listDevicePoliciesForSiteToSiteNatCheck = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{
			{
				ID:       "pol-3",
				Name:     "not-cross",
				Enabled:  true,
				RuleType: models.DevicePolicy,
				Src:      []models.AclPolicyTag{{ID: models.EgressID, Value: "eg-a"}},
				Dst:      []models.AclPolicyTag{{ID: models.EgressID, Value: "eg-a"}},
			},
		}
	}
	if err := ValidateEgressNatAllowedForSiteToSitePolicies("eg-a", "net1", true); err != nil {
		t.Fatalf("same-egress policy: expected nil, got %v", err)
	}

	listDevicePoliciesForSiteToSiteNatCheck = func(netID schema.NetworkID) []models.Acl {
		return []models.Acl{
			{
				ID:       "pol-4",
				Name:     "other-egress",
				Enabled:  true,
				RuleType: models.DevicePolicy,
				Src:      []models.AclPolicyTag{{ID: models.EgressID, Value: "eg-x"}},
				Dst:      []models.AclPolicyTag{{ID: models.EgressID, Value: "eg-y"}},
			},
		}
	}
	if err := ValidateEgressNatAllowedForSiteToSitePolicies("eg-a", "net1", true); err != nil {
		t.Fatalf("policy does not reference eg-a: expected nil, got %v", err)
	}
}
