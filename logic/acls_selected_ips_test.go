package logic

import (
	"errors"
	"testing"

	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func TestNormalizeAndValidateAclEgressIPs(t *testing.T) {
	originalGetEgressByID := getEgressByID
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
	})

	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "eg-1":
			return schema.Egress{ID: egressID, Range: "10.10.0.0/24"}, nil
		default:
			return schema.Egress{}, errors.New("not found")
		}
	}

	acl := models.Acl{
		Dst: []models.AclPolicyTag{
			{ID: models.EgressID, Value: "eg-1"},
			{ID: models.NetmakerIPAclID, Value: "10.10.0.10"},
		},
	}
	if err := NormalizeAndValidateAclEgressIPs(&acl); err != nil {
		t.Fatalf("expected valid selected ip, got error: %v", err)
	}
	if acl.Dst[1].Value != "10.10.0.10/32" {
		t.Fatalf("expected normalized host CIDR, got %s", acl.Dst[1].Value)
	}
}

func TestNormalizeAndValidateAclEgressIPsRejectsOutsideRange(t *testing.T) {
	originalGetEgressByID := getEgressByID
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
	})

	getEgressByID = func(egressID string) (schema.Egress, error) {
		return schema.Egress{ID: egressID, Range: "10.10.0.0/24"}, nil
	}

	acl := models.Acl{
		Dst: []models.AclPolicyTag{
			{ID: models.EgressID, Value: "eg-1"},
			{ID: models.NetmakerIPAclID, Value: "10.20.0.10"},
		},
	}
	if err := NormalizeAndValidateAclEgressIPs(&acl); err == nil {
		t.Fatal("expected error for selected ip outside egress range")
	}
}

func TestNormalizeAndValidateAclEgressIPsRequiresEgressReference(t *testing.T) {
	acl := models.Acl{
		Dst: []models.AclPolicyTag{
			{ID: models.NetmakerIPAclID, Value: "10.10.0.10"},
		},
	}
	if err := NormalizeAndValidateAclEgressIPs(&acl); err == nil {
		t.Fatal("expected error when egress ip is configured without egress destination")
	}
}

func TestNormalizeAndValidateAclEgressIPsOnSource(t *testing.T) {
	originalGetEgressByID := getEgressByID
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
	})

	getEgressByID = func(egressID string) (schema.Egress, error) {
		return schema.Egress{ID: egressID, Range: "10.11.0.0/24"}, nil
	}

	acl := models.Acl{
		Src: []models.AclPolicyTag{
			{ID: models.EgressID, Value: "eg-src"},
			{ID: models.NetmakerIPAclID, Value: "10.11.0.20"},
		},
	}
	if err := NormalizeAndValidateAclEgressIPs(&acl); err != nil {
		t.Fatalf("expected valid source selected ip, got error: %v", err)
	}
	if acl.Src[1].Value != "10.11.0.20/32" {
		t.Fatalf("expected normalized host CIDR, got %s", acl.Src[1].Value)
	}
}

func TestExpandAclEgressTagValuesIncludesSourceAndDestinationNodes(t *testing.T) {
	originalGetEgressByID := getEgressByID
	t.Cleanup(func() {
		getEgressByID = originalGetEgressByID
	})

	getEgressByID = func(egressID string) (schema.Egress, error) {
		switch egressID {
		case "eg-src":
			return schema.Egress{ID: egressID, Status: true, Nodes: datatypes.JSONMap{"src-node": float64(1)}}, nil
		case "eg-dst":
			return schema.Egress{ID: egressID, Status: true, Nodes: datatypes.JSONMap{"dst-node": float64(1)}}, nil
		default:
			return schema.Egress{}, errors.New("not found")
		}
	}

	srcMap := map[string]struct{}{}
	dstMap := map[string]struct{}{}
	ExpandAclEgressTagValues(
		srcMap,
		dstMap,
		[]models.AclPolicyTag{{ID: models.EgressID, Value: "eg-src"}},
		[]models.AclPolicyTag{{ID: models.EgressID, Value: "eg-dst"}},
	)

	if _, ok := srcMap["src-node"]; !ok {
		t.Fatal("expected src-node to be expanded from source egress")
	}
	if _, ok := dstMap["dst-node"]; !ok {
		t.Fatal("expected dst-node to be expanded from destination egress")
	}
}
