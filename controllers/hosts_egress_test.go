package controller

import (
	"testing"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/schema"
	"gorm.io/datatypes"
)

func TestShouldApplyEgressDomainAnsUpdate(t *testing.T) {
	tests := []struct {
		name     string
		egress   schema.Egress
		ranges   []string
		expected bool
	}{
		{
			name:     "applies when ranges exist",
			ranges:   []string{"10.0.0.0/24"},
			expected: true,
		},
		{
			name:     "skips when no ranges",
			ranges:   nil,
			expected: false,
		},
		{
			name: "skips aws preset with server-seeded domain answers",
			egress: schema.Egress{
				PresetID: "aws-s3-us-east-1",
				DomainAnsByDomain: datatypes.JSONMap{
					"s3.us-east-1.amazonaws.com": []interface{}{"52.216.0.0/15"},
				},
			},
			ranges:   []string{"10.0.0.0/24"},
			expected: false,
		},
		{
			name: "skips azure preset with server-seeded domain answers",
			egress: schema.Egress{
				PresetID: "azure-storage-eastus",
				DomainAnsByDomain: datatypes.JSONMap{
					"eastus.blob.core.windows.net": []interface{}{"13.64.0.0/11"},
				},
			},
			ranges:   []string{"10.0.0.0/24"},
			expected: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := shouldApplyEgressDomainAnsUpdate(tc.egress, tc.ranges)
			if got != tc.expected {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestShouldApplyEgressDomainAnsUpdate_AWSGuardUsesHasEgressDomainAns(t *testing.T) {
	e := schema.Egress{PresetID: "aws-ec2-us-west-2"}
	if !shouldApplyEgressDomainAnsUpdate(e, []string{"10.0.0.0/8"}) {
		t.Fatal("expected apply when aws preset has no domain answers yet")
	}
	logic.SetEgressDomainAnsForDomain(&e, "us-west-2.compute.amazonaws.com", []string{"10.0.0.0/8"})
	if shouldApplyEgressDomainAnsUpdate(e, []string{"192.168.0.0/24"}) {
		t.Fatal("expected skip when aws preset already has domain answers")
	}
}

func TestShouldApplyEgressDomainAnsUpdate_AzureGuardUsesHasEgressDomainAns(t *testing.T) {
	e := schema.Egress{PresetID: "azure-storage-eastus"}
	if !shouldApplyEgressDomainAnsUpdate(e, []string{"10.0.0.0/8"}) {
		t.Fatal("expected apply when azure preset has no domain answers yet")
	}
	logic.SetEgressDomainAnsForDomain(&e, "eastus.blob.core.windows.net", []string{"13.64.0.0/11"})
	if shouldApplyEgressDomainAnsUpdate(e, []string{"192.168.0.0/24"}) {
		t.Fatal("expected skip when azure preset already has domain answers")
	}
}
